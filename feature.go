package coalmine

import (
	"context"
	"hash/fnv"
)

// Feature represents a unit of functionality that can be enabled and disabled.
type Feature struct {
	name     string
	matchers []*Matcher
}

// Enabled returns true if the feature should be enabled given the current context.
func (f *Feature) Enabled(ctx context.Context) bool {
	if enabled, present := checkOverride(ctx, f.name); present {
		return enabled
	}
	if ks := getKillswitch(ctx); ks != nil && ks.Enabled(ctx, f.name) {
		return false
	}
	for _, matcher := range f.matchers {
		if matcher.evaluate(ctx) {
			return true
		}
	}
	return false
}

// NewFeature allocates a new Feature using the provided matcher options.
func NewFeature(name string, opts ...MatcherOption) *Feature {
	f := &Feature{
		name:     name,
		matchers: make([]*Matcher, len(opts)),
	}
	for i, opt := range opts {
		m := &Matcher{}
		opt(m)
		f.matchers[i] = m
	}
	return f
}

// Matcher is used to decide whether a feature should be enabled.
type Matcher struct {
	matchers []*Matcher
	fn       func(context.Context) bool
}

func (m *Matcher) evaluate(ctx context.Context) bool {
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

// MatcherOption configures a Matcher.
type MatcherOption func(*Matcher)

// WithAND configures a matcher that only evaluates to true when all of its child
// matchers do as well.
func WithAND(opts ...MatcherOption) MatcherOption {
	return func(m *Matcher) {
		m.matchers = make([]*Matcher, len(opts))
		for i, opt := range opts {
			child := &Matcher{}
			opt(child)
			m.matchers[i] = child
		}
	}
}

// WithExactMatch configures a matcher that only evaluates to true when an equal
// value for the given key/value pair has been set on the context with WithValue.
func WithExactMatch(key Key, value string) MatcherOption {
	return func(m *Matcher) {
		m.fn = func(ctx context.Context) bool {
			return getValue(ctx, key) == value
		}
	}
}

// WithPercentage configures a matcher that evaluates to true for a percent of the
// given key as set by WithValue.
func WithPercentage(key Key, percent uint32) MatcherOption {
	return func(m *Matcher) {
		m.fn = func(ctx context.Context) bool {
			h := fnv.New32a()
			h.Write([]byte(getValue(ctx, key)))
			return h.Sum32()%100 < percent
		}
	}
}
