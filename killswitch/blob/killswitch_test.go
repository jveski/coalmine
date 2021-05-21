package blob

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKillswitchTick(t *testing.T) {
	ctx := context.Background()
	content := "foo\nbar\n"
	store := BlobStoreFn(func(ctx context.Context, url string) ([]byte, error) {
		return []byte(content), nil
	})

	k := New(store)
	require.NoError(t, k.tick(ctx))
	assert.True(t, k.Enabled(ctx, "foo"))
	assert.True(t, k.Enabled(ctx, "bar"))
	assert.False(t, k.Enabled(ctx, "baz"))
}

func TestKillswitchFullIntegration(t *testing.T) {
	content := "foo\nbar\n"
	testURL := "test-url"
	var storeError error
	store := BlobStoreFn(func(ctx context.Context, url string) ([]byte, error) {
		assert.Equal(t, testURL, url)
		return []byte(content), storeError
	})

	errorHandled := false
	errHandler := func(ctx context.Context, err error) {
		assert.Equal(t, storeError, errors.Unwrap(err))
		errorHandled = true
	}

	ctx, done := context.WithCancel(context.Background())
	defer done()

	k := New(store,
		WithURL(testURL),
		WithErrorHandler(errHandler),
		WithInterval(time.Millisecond*2, time.Millisecond))
	require.NoError(t, k.Start(ctx))
	assert.True(t, k.Enabled(ctx, "foo"))
	assert.True(t, k.Enabled(ctx, "bar"))

	// Eventually changes should be noticed
	content = "foo\n"
	for {
		if !k.Enabled(ctx, "bar") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	assert.True(t, k.Enabled(ctx, "foo"))

	// The error handler should be called for errors
	storeError = errors.New("test")
	for {
		if errorHandled {
			break
		}
		time.Sleep(time.Millisecond)
	}
}
