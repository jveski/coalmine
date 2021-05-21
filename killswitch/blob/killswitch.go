package blob

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/jveski/coalmine/killswitch"
)

var (
	failedReqMetric = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "coalmine_killswitch_blob_errors_total",
		Help: "Number of times a request to get killswitch state from blob storage has failed.",
	})
)

func init() {
	prometheus.MustRegister(failedReqMetric)
}

// BlobStore is a generic blob store interface that can easily be implemented
// using S3, Azure blob storage, etc.
type BlobStore interface {
	GetBlobData(ctx context.Context, url string) ([]byte, error)
}

// BlobStoreFn implements BlobStore as a simple function.
type BlobStoreFn func(ctx context.Context, url string) ([]byte, error)

func (b BlobStoreFn) GetBlobData(ctx context.Context, url string) ([]byte, error) { return b(ctx, url) }

type KillswitchOption func(*Killswitch)

// WithInterval overrides the default polling interval and jitter.
func WithInterval(interval, jitter time.Duration) KillswitchOption {
	return func(k *Killswitch) {
		k.interval = interval
		k.jitter = jitter
	}
}

// WithErrorHandler sets the error handling hook. Allows blob store errors to be logged.
func WithErrorHandler(fn func(context.Context, error)) KillswitchOption {
	return func(k *Killswitch) {
		k.errorHandler = fn
	}
}

// WithURL adds a URL that will be passed to the blob store.
func WithURL(url string) KillswitchOption {
	return func(k *Killswitch) {
		k.url = url
	}
}

// Killswitch implements the killswitch interface using generic blob storage.
type Killswitch struct {
	store            BlobStore
	url              string
	errorHandler     func(context.Context, error)
	interval, jitter time.Duration
	mut              sync.Mutex
	state            map[string]struct{}
}

var _ killswitch.Killswitch = (*Killswitch)(nil)

// New constructs a new blob-backed killswitch implementation. Expects to get back
// a string from the blob store containing one feature name per line.
func New(store BlobStore, opts ...KillswitchOption) *Killswitch {
	k := &Killswitch{
		store:    store,
		interval: time.Second * 30,
		jitter:   time.Second * 5,
		state:    map[string]struct{}{},
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

// Start runs the blob store polling loop. Blocks until the first poll is successful
// and returns an error otherwise.
func (k *Killswitch) Start(ctx context.Context) error {
	ticker := time.NewTicker(k.jitterTime())
	if err := k.tick(ctx); err != nil {
		return err
	}
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := k.tick(ctx); err != nil && k.errorHandler != nil {
					k.errorHandler(ctx, err)
				}
			case <-ctx.Done():
				return
			}
			ticker.Reset(k.jitterTime())
		}
	}()
	return nil
}

func (k *Killswitch) jitterTime() time.Duration {
	// avoid panic when passing 0 to rand.Intn
	if k.jitter == 0 {
		return k.interval
	}
	return k.interval + time.Duration(rand.Intn(int(k.jitter)))
}

func (k *Killswitch) tick(ctx context.Context) error {
	raw, err := k.store.GetBlobData(ctx, k.url)
	if err != nil {
		failedReqMetric.Inc()
		return fmt.Errorf("getting blob data: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	m := map[string]struct{}{}
	for _, line := range lines {
		m[line] = struct{}{}
	}

	k.mut.Lock()
	defer k.mut.Unlock()
	k.state = m
	return nil
}

// Enabled implements the killswitch interface.
func (k *Killswitch) Enabled(ctx context.Context, feature string) bool {
	k.mut.Lock()
	defer k.mut.Unlock()
	_, ok := k.state[feature]
	return ok
}
