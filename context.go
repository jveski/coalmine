package coalmine

import (
	"context"
	"strings"
	"time"

	"github.com/jveski/coalmine/internal/killswitch"
)

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
