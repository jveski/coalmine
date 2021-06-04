package killswitch

import (
	"context"
	"io/ioutil"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoopRunningIntegration(t *testing.T) {
	l, update := newTestLoop(t, "foo=1")
	initialHash := l.hash

	// Prove hash changes when the file changse
	require.NoError(t, update("foo=2"))
	for {
		if lvl, _ := l.Get("foo"); lvl == 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	assert.NotEqual(t, initialHash, l.hash)

	// Prove hash is stable when file doesn't change
	initialHash = l.hash
	time.Sleep(l.interval * 2)
	assert.Equal(t, initialHash, l.hash)
}

func TestLoopEnabledIntegration(t *testing.T) {
	l, _ := newTestLoop(t, "foo=0\nbaz\nbar=2")

	lvl, ok := l.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, int64(0), lvl)

	lvl, ok = l.Get("bar")
	assert.True(t, ok)
	assert.Equal(t, int64(2), lvl)

	lvl, ok = l.Get("baz")
	assert.True(t, ok)
	assert.Equal(t, int64(math.MaxInt64), lvl)
}

func TestLoopIntervalJitter(t *testing.T) {
	l, _ := newTestLoop(t, "")

	pass := false
	var prev time.Duration
	for i := 0; i < 10; i++ {
		v := l.intervalJitter()
		t.Logf("interval jitter: %s", v)
		if i > 1 && v != prev {
			pass = true
		}
		prev = v
	}
	if !pass {
		t.Error("interval didn't include jitter")
	}
}

func newTestLoop(t *testing.T, init string) (*Loop, func(string) error) {
	file, err := ioutil.TempFile("", "")
	require.NoError(t, err)

	t.Cleanup(func() {
		file.Close()
		if err := os.Remove(file.Name()); err != nil {
			panic(err)
		}
	})

	l := NewLoop(file.Name(), time.Millisecond*5)
	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(func() {
		done()
	})

	write := func(content string) error {
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}
		if err := file.Truncate(0); err != nil {
			return err
		}
		if _, err := file.Write([]byte(content)); err != nil {
			return err
		}
		return nil
	}
	require.NoError(t, write(init))
	l.Start(ctx)

	return l, write
}
