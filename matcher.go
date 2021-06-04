package coalmine

import (
	"context"
	"hash/fnv"
)

// MatcherOption configures matchers: logical operations against context values set by WithValue.
type MatcherOption func(*Feature) *matcher

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
