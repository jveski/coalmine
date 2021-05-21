package coalmine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jveski/coalmine/killswitch"
)

func TestFeatureNoMatchers(t *testing.T) {
	f := NewFeature(t.Name())
	assert.False(t, f.Enabled(context.Background()))
}

func TestFeatureExactMatch(t *testing.T) {
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	t.Run("positive", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("wrong value", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, "wrong value")
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("missing value", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeaturePercentage(t *testing.T) {
	key := Key("test-key")
	f := NewFeature(t.Name(), WithPercentage(key, 50))

	t.Run("positive", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, "1")
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("negative", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, "3")
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureMatchOR(t *testing.T) {
	key, value, value2 := Key("test-key"), "test-value", "test-value-2"
	f := NewFeature(t.Name(), WithExactMatch(key, value), WithExactMatch(key, value2))

	t.Run("first", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("second", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("both", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		ctx = WithValue(ctx, key, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("neither", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureMatchAND(t *testing.T) {
	key, key2, value, value2 := Key("test-key"), Key("test-key-2"), "test-value", "test-value-2"
	f := NewFeature(t.Name(), WithAND(WithExactMatch(key, value), WithExactMatch(key2, value2)))

	t.Run("first", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("second", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key2, value2)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("both", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		ctx = WithValue(ctx, key2, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("neither", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureOverride(t *testing.T) {
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	t.Run("feature on, override off", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		ctx = WithOverride(ctx, f, false)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("feature on, override on", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithValue(ctx, key, value)
		ctx = WithOverride(ctx, f, true)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("feature off, override off", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithOverride(ctx, f, false)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("feature off, override on", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithOverride(ctx, f, true)
		assert.True(t, f.Enabled(ctx))
	})
}

func TestFeatureKillswitch(t *testing.T) {
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	ks := killswitch.NewMemory()
	ks.Set(t.Name())

	ctx := context.Background()
	ctx = WithKillswitch(ctx, ks)
	ctx = WithValue(ctx, key, value)
	assert.False(t, f.Enabled(ctx))
}
