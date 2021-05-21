package coalmine

import (
	"context"

	"github.com/jveski/coalmine/killswitch"
)

type valueKey string

type overrideKey string

type killswitchKey struct{}

// WithOverride forces the given feature to be either enabled or disabled. Useful in tests.
func WithOverride(ctx context.Context, feature *Feature, enable bool) context.Context {
	return context.WithValue(ctx, overrideKey(feature.name), enable)
}

// WithValue adds a string kv pair to the context for use with matchers.
func WithValue(ctx context.Context, key Key, value string) context.Context {
	return context.WithValue(ctx, valueKey(key), value)
}

// WithKillswitch registers a killswitch.
func WithKillswitch(ctx context.Context, ks killswitch.Killswitch) context.Context {
	return context.WithValue(ctx, killswitchKey{}, ks)
}

func getValue(ctx context.Context, key Key) string {
	val := ctx.Value(valueKey(key))
	if val == nil {
		return ""
	}
	return val.(string)
}

func checkOverride(ctx context.Context, feature string) (bool /* state */, bool /* present */) {
	val := ctx.Value(overrideKey(feature))
	if val == nil {
		return false, false
	}
	return val.(bool), true
}

func getKillswitch(ctx context.Context) killswitch.Killswitch {
	val := ctx.Value(killswitchKey{})
	if val == nil {
		return nil
	}
	return val.(killswitch.Killswitch)
}
