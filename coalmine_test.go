package coalmine

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureNoMatchers(t *testing.T) {
	f := NewFeature(t.Name())
	assert.False(t, f.Enabled(context.Background()))
}

func TestFeatureExactMatch(t *testing.T) {
	ctx := context.Background()
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	t.Run("positive", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("wrong casing", func(t *testing.T) {
		ctx := WithValue(ctx, Key("TEST-KEY"), value)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("wrong value", func(t *testing.T) {
		ctx := WithValue(ctx, key, "wrong value")
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("missing value", func(t *testing.T) {
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeaturePercentage(t *testing.T) {
	ctx := context.Background()
	key := Key("test-key")
	f := NewFeature(t.Name(), WithPercentage(key, 50))

	t.Run("positive", func(t *testing.T) {
		ctx := WithValue(ctx, key, "1")
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("wrong casing", func(t *testing.T) {
		ctx := WithValue(ctx, Key("TEST-KEY"), "1")
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("negative", func(t *testing.T) {
		ctx := WithValue(ctx, key, "3")
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureMatchOR(t *testing.T) {
	ctx := context.Background()
	key, value, value2 := Key("test-key"), "test-value", "test-value-2"
	f := NewFeature(t.Name(), WithExactMatch(key, value), WithExactMatch(key, value2))

	t.Run("first", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("second", func(t *testing.T) {
		ctx := WithValue(ctx, key, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("both", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		ctx = WithValue(ctx, key, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("neither", func(t *testing.T) {
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureMatchAND(t *testing.T) {
	ctx := context.Background()
	key, key2, value, value2 := Key("test-key"), Key("test-key-2"), "test-value", "test-value-2"
	f := NewFeature(t.Name(), WithAND(WithExactMatch(key, value), WithExactMatch(key2, value2)))

	t.Run("first", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("second", func(t *testing.T) {
		ctx := WithValue(ctx, key2, value2)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("both", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		ctx = WithValue(ctx, key2, value2)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("neither", func(t *testing.T) {
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureOverride(t *testing.T) {
	ctx := context.Background()
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	t.Run("feature on, override off", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		ctx = WithOverride(ctx, f, false)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("feature on, override on", func(t *testing.T) {
		ctx := WithValue(ctx, key, value)
		ctx = WithOverride(ctx, f, true)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("feature off, override off", func(t *testing.T) {
		ctx := WithOverride(ctx, f, false)
		assert.False(t, f.Enabled(ctx))
	})

	t.Run("feature off, override on", func(t *testing.T) {
		ctx := WithOverride(ctx, f, true)
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("prove other feature not matched", func(t *testing.T) {
		ctx := WithOverride(ctx, f, true)

		f2 := NewFeature("some other feature")
		assert.False(t, f2.Enabled(ctx))
	})
}

func TestFeatureOverrideString(t *testing.T) {
	ctx := context.Background()

	t.Run("positive", func(t *testing.T) {
		f := NewFeature(t.Name())
		ctx := WithOverrideString(ctx, "Foo", "Foo1,Foo"+t.Name())
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("wrong casing", func(t *testing.T) {
		f := NewFeature(t.Name())
		ctx := WithOverrideString(ctx, "Foo", "Foo1,Foo"+strings.ToUpper(t.Name()))
		assert.True(t, f.Enabled(ctx))
	})

	t.Run("negative", func(t *testing.T) {
		f := NewFeature(t.Name())
		ctx := WithOverrideString(ctx, "Foo", "Foo1,FooAlso")
		assert.False(t, f.Enabled(ctx))
	})
}

func TestFeatureKillswitch(t *testing.T) {
	key, value := Key("test-key"), "test-value"
	f := NewFeature(t.Name(), WithExactMatch(key, value))

	os.Setenv("COALMINE_KILLSWITCH", "someotherfeature,"+t.Name()+",anotherfeature")
	defer os.Setenv("COALMINE_KILLSWITCH", "")
	warmKillswitchCache()

	ctx := context.Background()
	ctx = WithValue(ctx, key, value)
	assert.False(t, f.Enabled(ctx))
}

func TestFeatureDuplicateName(t *testing.T) {
	NewFeature(t.Name())
	assert.Panics(t, func() {
		NewFeature(t.Name())
	})
}

func TestFeatureObserver(t *testing.T) {
	ctx := context.Background()
	f := NewFeature(t.Name())

	t.Run("enabled", func(t *testing.T) {
		ctx := WithOverride(ctx, f, true)
		called := false
		ctx = WithObserver(ctx, func(ctx context.Context, feat string, state bool) {
			called = true
			assert.Equal(t, f.name, feat)
			assert.True(t, state)
		})
		f.Enabled(ctx)
		assert.True(t, called)
	})

	t.Run("disabled", func(t *testing.T) {
		called := false
		ctx := WithObserver(ctx, func(ctx context.Context, feat string, state bool) {
			called = true
			assert.Equal(t, f.name, feat)
			assert.False(t, state)
		})
		f.Enabled(ctx)
		assert.True(t, called)
	})
}
