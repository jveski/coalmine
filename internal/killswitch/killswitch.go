package killswitch

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// TODO: Allow multiple defs

var (
	infoMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "coalmine_killswitch_info",
			Help: "Metadata related to the coalmine killswitch.",
		},
		[]string{"hash"},
	)
)

func init() {
	prometheus.MustRegister(infoMetric)
}

type Loop struct {
	file     string
	interval time.Duration

	mut   sync.Mutex
	cache map[string]int64
	hash  string
}

func NewLoop(file string, interval time.Duration) *Loop {
	return &Loop{
		file:     file,
		interval: interval,
		cache:    map[string]int64{},
	}
}

func (l *Loop) Get(name string) (int64, bool) {
	l.mut.Lock()
	defer l.mut.Unlock()
	val, ok := l.cache[name]
	return val, ok
}

func (l *Loop) Start(ctx context.Context) {
	l.tick()

	go func() {
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				l.tick()
				ticker.Reset(l.intervalJitter())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (l *Loop) tick() {
	f, err := os.Open(l.file)
	if err != nil {
		return
	}
	defer f.Close()

	hasher := sha256.New()
	tee := io.TeeReader(f, hasher)
	scanner := bufio.NewScanner(tee)
	m := map[string]int64{}
	for scanner.Scan() {
		chunks := strings.Split(scanner.Text(), "=")
		var level int64
		if len(chunks) > 1 {
			if lvl, err := strconv.ParseInt(chunks[1], 10, 0); err == nil {
				level = lvl
			}
		} else {
			level = -math.MinInt64 - 1
		}
		m[strings.ToLower(chunks[0])] = level
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	func() {
		l.mut.Lock()
		defer l.mut.Unlock()
		l.hash = hash
		l.cache = m
	}()

	infoMetric.Reset()
	infoMetric.WithLabelValues(hash).Set(1)
}

func (l *Loop) intervalJitter() time.Duration {
	swing := int(l.interval / 10)
	if swing < 1 {
		return l.interval // avoid rand.Intn panic for 0 value
	}
	return l.interval + time.Duration(rand.Intn(swing)-(swing/2))
}
