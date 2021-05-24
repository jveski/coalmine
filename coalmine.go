// Package coalmine helps get features into production safely using canaries
package coalmine

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"sync"

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

var killswitchCache = sync.Map{}

func warmKillswitchCache() {
	for _, feat := range strings.Split(os.Getenv("COALMINE_KILLSWITCH"), ",") {
		killswitchCache.Store(feat, struct{}{})
	}
}

func init() {
	prometheus.MustRegister(enabledMetric)
	warmKillswitchCache()
}

// Feature represents a unit of functionality that can be enabled and disabled.
type Feature struct {
	name     string
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
	if enabled, present := getOverride(ctx, f.name); present {
		ok = enabled
		return ok
	}
	if _, enabled := killswitchCache.Load(f.name); enabled {
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
	if _, ok := featureNames.LoadOrStore(name, struct{}{}); ok {
		panic(fmt.Errorf("a coalmine feature with the name %q already exists", name))
	}
	f := &Feature{
		name:     name,
		matchers: make([]*matcher, len(opts)),
	}
	for i, opt := range opts {
		m := &matcher{}
		opt(m)
		f.matchers[i] = m
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

// Key is the key of a key/value pair that can be matched on using a Matcher.
type Key string

// MatcherOption configures matchers: logical operations against context values set by WithValue.
type MatcherOption func(*matcher)

// WithAND enables a feature when all child matchers are positively matched.
func WithAND(opts ...MatcherOption) MatcherOption {
	return func(m *matcher) {
		m.matchers = make([]*matcher, len(opts))
		for i, opt := range opts {
			child := &matcher{}
			opt(child)
			m.matchers[i] = child
		}
	}
}

// WithExactMatch enables a feature when a string value passes an equality check
// against the corresponding context value.
func WithExactMatch(key Key, value string) MatcherOption {
	return func(m *matcher) {
		m.fn = func(ctx context.Context) bool {
			return getValue(ctx, key) == value
		}
	}
}

// WithPercentage enables a feature for a percent of the possible values of a given context key.
// Uses Go's Fowler–Noll–Vo hash implementation (hash/fnv.New32a).
func WithPercentage(key Key, percent uint32) MatcherOption {
	return func(m *matcher) {
		m.fn = func(ctx context.Context) bool {
			h := fnv.New32a()
			h.Write([]byte(getValue(ctx, key)))
			return h.Sum32()%100 < percent
		}
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

// WithOverrideString forces a given list of feature to be enabled. List is specified as a comma-separated
// string (str) and optional prefix to be removed from each item (prfx).
func WithOverrideString(ctx context.Context, prfx, str string) context.Context {
	for _, chunk := range strings.Split(str, ",") {
		cleaned := strings.TrimPrefix(chunk, prfx)
		ctx = context.WithValue(ctx, newFeatureKey(cleaned), true)
	}
	return ctx
}

type valueKey string

func newValueKey(key Key) valueKey { return valueKey(strings.ToLower(string(key))) }

// WithValue adds a string kv pair to the context for use with matchers.
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
