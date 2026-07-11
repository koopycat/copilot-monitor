package proxy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"copilot-monitoring/internal/store"
)

// dedupWindow is how long to suppress duplicate anomalies for the same
// (category, path, detail_hash) tuple.
const dedupWindow = 5 * time.Minute

// anomalyChannelSize is the buffer capacity for the background write channel.
const anomalyChannelSize = 1024

// AnomalyRecorder receives anomaly records from detection hooks on the hot path
// and writes them to the store via a background goroutine. It does not block
// the caller unless the channel is full (in which case the record is dropped).
type AnomalyRecorder struct {
	store    *store.Store
	ch       chan store.AnomalyRecord
	dedup    sync.Map // key: dedupKey -> time.Time of last write
	stopped  chan struct{}
	once     sync.Once
	mu       sync.Mutex
	shutdown bool
	dropped  atomic.Int64
}

type dedupKey struct {
	category string
	path     string
	hash     string
}

// NewAnomalyRecorder creates an AnomalyRecorder, starts a background goroutine
// that reads from the channel and persists anomalies to the store.
func NewAnomalyRecorder(st *store.Store) *AnomalyRecorder {
	r := &AnomalyRecorder{
		store: st,
	}
	if st != nil {
		r.ch = make(chan store.AnomalyRecord, anomalyChannelSize)
		r.stopped = make(chan struct{})
		go r.run()
		go r.dedupCleanup()
		return r
	}
	stopped := make(chan struct{})
	close(stopped)
	r.stopped = stopped
	r.shutdown = true
	return r
}

func (r *AnomalyRecorder) run() {
	defer close(r.stopped)
	for rec := range r.ch {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := r.store.WriteAnomaly(ctx, rec); err != nil {
			fmt.Fprintf(os.Stderr, "anomaly write failed: %v\n", err)
		}
		cancel()
	}
}

// Record attempts to record an anomaly. It first checks deduplication, then
// sends to the buffered channel. If the channel is full, the record is
// silently dropped. This method never blocks the caller.
func (r *AnomalyRecorder) Record(rec store.AnomalyRecord) {
	if r.store == nil {
		return
	}

	key := dedupKey{
		category: rec.Category,
		path:     rec.Path,
		hash:     hashDetail(rec.Detail),
	}

	// Deduplication: suppress identical records within the cooldown window.
	if v, ok := r.dedup.Load(key); ok {
		if time.Since(v.(time.Time)) < dedupWindow {
			return
		}
	}

	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return
	}

	select {
	case r.ch <- rec:
		r.dedup.Store(key, time.Now())
	default:
		r.dropped.Add(1)
	}
	r.mu.Unlock()
}

// Shutdown stops the background goroutine and drains remaining records from
// the channel. It blocks until the goroutine exits. Safe to call multiple
// times (idempotent).
func (r *AnomalyRecorder) Shutdown() {
	if r.store == nil {
		return
	}
	r.once.Do(func() {
		r.mu.Lock()
		r.shutdown = true
		close(r.ch)
		r.mu.Unlock()
		<-r.stopped
	})
}

func hashDetail(detail string) string {
	if detail == "" {
		return ""
	}
	h := sha256.Sum256([]byte(detail))
	return fmt.Sprintf("%x", h[:])
}

func (r *AnomalyRecorder) dedupCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	cutoff := dedupWindow + 1*time.Minute
	for {
		select {
		case <-ticker.C:
			threshold := time.Now().Add(-cutoff)
			r.dedup.Range(func(key, value any) bool {
				if value.(time.Time).Before(threshold) {
					r.dedup.Delete(key)
				}
				return true
			})
		case <-r.stopped:
			return
		}
	}
}

// Dropped returns the number of records dropped due to a full channel.
func (r *AnomalyRecorder) Dropped() int64 {
	return r.dropped.Load()
}
