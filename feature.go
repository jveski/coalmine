// Package coalmine helps get features into production safely using canaries
package coalmine

import (
	"context"
	"hash/fnv"
	"os"
	"strings"

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

	killswitchMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coalmine_feature_killswitch_total",
			Help: "Number of times a feature is disabled by a killswitch.",
		},
		[]string{"feature"},
	)
)

func init() {
	prometheus.MustRegister(enabledMetric)
	prometheus.MustRegister(killswitchMetric)
}

// Feature represents a unit of functionality that can be enabled and disabled.
type Feature struct {
	name     string
	matchers []*matcher
}

// Enabled returns true if the feature should be enabled given the current context.
func (f *Feature) Enabled(ctx context.Context) bool {
	if enabled, present := getFeatureOverride(ctx, f.name); present {
		return enabled
	}
	if enabled, present := getGlobalOverride(ctx); present {
		return enabled
	}
	if checkKillswitch(f.name) {
		killswitchMetric.WithLabelValues(f.name).Inc()
		return false
	}
	for _, matcher := range f.matchers {
		if matcher.evaluate(ctx) {
			enabledMetric.WithLabelValues(f.name).Inc()
			return true
		}
	}
	return false
}

// NewFeature allocates a new Feature using the provided matcher options.
func NewFeature(name string, opts ...MatcherOption) *Feature {
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

func checkKillswitch(name string) bool {
	for _, feat := range strings.Split(os.Getenv("COALMINE_KILLSWITCH"), ",") {
		if feat == name {
			return true
		}
	}
	return false
}
