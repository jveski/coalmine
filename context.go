package coalmine

import (
	"context"

	"github.com/jveski/coalmine/killswitch"
)

type featureOverrideKey string

// WithFeatureOverride forces the given feature to be either enabled or disabled. Useful in tests.
func WithFeatureOverride(ctx context.Context, feature *Feature, enable bool) context.Context {
	return context.WithValue(ctx, featureOverrideKey(feature.name), enable)
}

func getFeatureOverride(ctx context.Context, feature string) (bool /* state */, bool /* present */) {
	val := ctx.Value(featureOverrideKey(feature))
	if val == nil {
		return false, false
	}
	return val.(bool), true
}

type globalOverrideKey struct{}

// WithGlobalOverride forces all features to be either enabled or disabled. Useful in tests.
func WithGlobalOverride(ctx context.Context, enable bool) context.Context {
	return context.WithValue(ctx, globalOverrideKey{}, enable)
}

func getGlobalOverride(ctx context.Context) (bool, bool) {
	val := ctx.Value(globalOverrideKey{})
	if val == nil {
		return false, false
	}
	return val.(bool), true
}

type valueKey string

// WithValue adds a string kv pair to the context for use with matchers.
func WithValue(ctx context.Context, key Key, value string) context.Context {
	return context.WithValue(ctx, valueKey(key), value)
}

func getValue(ctx context.Context, key Key) string {
	val := ctx.Value(valueKey(key))
	if val == nil {
		return ""
	}
	return val.(string)
}

type killswitchKey struct{}

// WithKillswitch registers a killswitch. See coalmine/killswitch documentation for more.
func WithKillswitch(ctx context.Context, ks killswitch.Killswitch) context.Context {
	return context.WithValue(ctx, killswitchKey{}, ks)
}

func getKillswitch(ctx context.Context) killswitch.Killswitch {
	val := ctx.Value(killswitchKey{})
	if val == nil {
		return nil
	}
	return val.(killswitch.Killswitch)
}
