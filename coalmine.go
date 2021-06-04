// Package coalmine helps get features into production safely using canaries
package coalmine

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/jveski/coalmine/internal/killswitch"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	enabledMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coalmine_feature_enable_total",
			Help: "Number of times a feature is enabled.",
		},
		[]string{"feature"},
	)
)

func init() {
	prometheus.MustRegister(enabledMetric)
}

// Feature represents a unit of functionality that can be enabled and disabled.
type Feature struct {
	name     string
	ksLevel  int64
	matchers []*matcher
}

// Enabled returns true if the feature should be enabled given the current context.
func (f *Feature) Enabled(ctx context.Context) (ok bool) {
	observer := getObserver(ctx)
	if observer != nil {
		defer func() {
			observer(ctx, f.name, ok)
		}()
	}
	if ks := getKillswitch(ctx); ks != nil {
		if lvl, ok := ks.Get(strings.ToLower(f.name)); ok && lvl > f.ksLevel {
			ok = false
			return ok
		}
	}
	if enabled, present := getOverride(ctx, f.name); present {
		ok = enabled
		return ok
	}
	for _, matcher := range f.matchers {
		if matcher.evaluate(ctx) {
			enabledMetric.WithLabelValues(f.name).Inc()
			ok = true
			return ok
		}
	}
	return ok
}

var featureNames = sync.Map{}

// NewFeature allocates a new Feature using the provided matcher options.
func NewFeature(name string, opts ...MatcherOption) *Feature {
	if _, ok := featureNames.LoadOrStore(strings.ToLower(name), struct{}{}); ok {
		panic(fmt.Errorf("a coalmine feature with the name %q already exists", name))
	}
	f := &Feature{
		name: name,
	}
	for _, opt := range opts {
		m := opt(f)
		if m != nil {
			f.matchers = append(f.matchers, m)
		}
	}
	return f
}

type matcher struct {
	matchers []*matcher
	fn       func(context.Context) bool
}

func (m *matcher) evaluate(ctx context.Context) bool {
	if m.fn != nil {
		return m.fn(ctx)
	}
	for _, child := range m.matchers {
		if !child.evaluate(ctx) {
			return false
		}
	}
	return true
}

// Key is a case-insensitive string key for context values used by coalmine.
type Key string

// MatcherOption configures matchers: logical operations against context values set by WithValue.
type MatcherOption func(*Feature) *matcher

// WithAND enables a feature when all child matchers are positively matched.
func WithAND(opts ...MatcherOption) MatcherOption {
	return func(f *Feature) *matcher {
		m := &matcher{}
		m.matchers = make([]*matcher, len(opts))
		for i, opt := range opts {
			child := opt(f)
			if child != nil {
				m.matchers[i] = child
			}
		}
		return m
	}
}

// WithExactMatch enables a feature when a string value passes an equality check
// against the corresponding context value.
func WithExactMatch(key Key, value string) MatcherOption {
	return func(f *Feature) *matcher {
		m := &matcher{}
		m.fn = func(ctx context.Context) bool {
			return getValue(ctx, key) == value
		}
		return m
	}
}

// WithPercentage enables a feature for a percent of the possible values of a given context key.
// Uses Go's Fowler–Noll–Vo hash implementation (hash/fnv.New32a).
func WithPercentage(key Key, percent uint32) MatcherOption {
	return func(f *Feature) *matcher {
		m := &matcher{}
		m.fn = func(ctx context.Context) bool {
			h := fnv.New32a()
			h.Write([]byte(getValue(ctx, key)))
			return h.Sum32()%100 < percent
		}
		return m
	}
}

// WithKillswitchOverride overrides an active killswitch for this feature by level.
// Overrides are active when the given override level > the killswitch.
//
// This allows features to be re-enabled during subsequent releases after being disabled at runtime.
func WithKillswitchOverride(level int64) MatcherOption {
	return func(f *Feature) *matcher {
		f.ksLevel = level
		return nil
	}
}

type featureKey string

func newFeatureKey(str string) featureKey { return featureKey(strings.ToLower(str)) }

// WithOverride forces the given feature to be either enabled or disabled. Useful in tests.
func WithOverride(ctx context.Context, feature *Feature, enable bool) context.Context {
	return context.WithValue(ctx, newFeatureKey(feature.name), enable)
}

func getOverride(ctx context.Context, feature string) (bool /* state */, bool /* present */) {
	val := ctx.Value(newFeatureKey(feature))
	if val == nil {
		return false, false
	}
	return val.(bool), true
}

// WithOverrideString forces a list of feature to be enabled. Specified as a comma-separated
// string and optional prefix to be removed from each item.
func WithOverrideString(ctx context.Context, prfx, str string) context.Context {
	for _, chunk := range strings.Split(str, ",") {
		cleaned := strings.TrimPrefix(chunk, prfx)
		ctx = context.WithValue(ctx, newFeatureKey(cleaned), true)
	}
	return ctx
}

type valueKey string

func newValueKey(key Key) valueKey { return valueKey(strings.ToLower(string(key))) }

// WithValue adds a string kv pair to the context for use with matchers. Keys are case-insensitive.
func WithValue(ctx context.Context, key Key, value string) context.Context {
	return context.WithValue(ctx, newValueKey(key), value)
}

func getValue(ctx context.Context, key Key) string {
	val := ctx.Value(newValueKey(key))
	if val == nil {
		return ""
	}
	return val.(string)
}

type observerKey struct{}

type ObserverFunc func(ctx context.Context, feature string, state bool)

// WithObserver registers a function to be called every time a feature is evaluated by feature.Enabled.
// Useful for logging feature states.
func WithObserver(ctx context.Context, fn ObserverFunc) context.Context {
	return context.WithValue(ctx, observerKey{}, fn)
}

func getObserver(ctx context.Context) ObserverFunc {
	val := ctx.Value(observerKey{})
	if val == nil {
		return nil
	}
	return val.(ObserverFunc)
}

type killswitchKey struct{}

// WithKillswitch periodically checks a killswitch file to disable features at runtime.
// Loop polls at the pollInterval with 10% jitter and returns when the context is done.
//
// The file referenced at path doesn't need to exist until it's needed.
// If it does exist, this function will block until it is read to avoid missing state at startup.
//
// The file should contain one feature name per line.
// If the killswitch should be overridable using WithKillswitchOverride, provide a level like feature=1.
// See WithKillswitchOverride for more details.
func WithKillswitch(ctx context.Context, path string, pollInterval time.Duration) context.Context {
	loop := killswitch.NewLoop(path, pollInterval)
	loop.Start(ctx)
	return context.WithValue(ctx, killswitchKey{}, loop)
}

func getKillswitch(ctx context.Context) *killswitch.Loop {
	val := ctx.Value(killswitchKey{})
	if val == nil {
		return nil
	}
	return val.(*killswitch.Loop)
}
