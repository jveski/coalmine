// Package coalmine helps get features into production safely using canaries
package coalmine

import (
	"context"
	"fmt"
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

var featureNames = sync.Map{}

func init() {
	prometheus.MustRegister(enabledMetric)
}

// Feature represents a unit of functionality that can be enabled and disabled.
type Feature struct {
	name     string
	ksLevel  int64
	matchers []*matcher
}

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

// Key is a case-insensitive string key for context values used by coalmine.
type Key string
