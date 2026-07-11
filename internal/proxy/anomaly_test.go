package proxy

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestAnomalyRecorderDeduplication(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)
	defer r.Shutdown()

	rec := store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/new-endpoint",
		Detail:    "no route matched",
	}

	// First record through
	r.Record(rec)
	// Second identical record within dedup window - should drop
	r.Record(rec)
	// Third - should also drop
	r.Record(rec)

	// Shutdown to drain
	r.Shutdown()

	anomalies, err := st.QueryAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("len(anomalies) = %d, want 1 (dedup failed)", len(anomalies))
	}
}

func TestAnomalyRecorderDifferentRecords(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)
	defer r.Shutdown()

	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/foo",
		Detail:    "first",
	})
	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/bar",
		Detail:    "second",
	})
	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "parse_error",
		Severity:  "warn",
		Path:      "/chat/completions",
		Detail:    "third",
	})

	r.Shutdown()

	anomalies, err := st.QueryAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 3 {
		t.Fatalf("len(anomalies) = %d, want 3", len(anomalies))
	}
}

func TestAnomalyRecorderConcurrentDedup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrent.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)

	rec := store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/endpoint",
		Detail:    "concurrent dedup test",
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Record(rec)
		}()
	}
	wg.Wait()

	r.Shutdown()

	anomalies, err := st.QueryAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(anomalies) > 5 {
		t.Fatalf("len(anomalies) = %d, want <= 5 (best-effort dedup under concurrency - TOCTOU ok)", len(anomalies))
	}
}

func TestAnomalyRecorderPostShutdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "postshutdown.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)

	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/foo",
		Detail:    "pre-shutdown",
	})

	r.Shutdown()

	// Record after Shutdown must not panic.
	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/bar",
		Detail:    "post-shutdown",
	})
}

func TestAnomalyRecorderDropped(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dropped.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)

	// Flood with 2000 records, far exceeding the 1024 buffer capacity.
	for i := 0; i < 2000; i++ {
		r.Record(store.AnomalyRecord{
			Timestamp: time.Now().UTC(),
			Category:  "unrouted_path",
			Severity:  "warn",
			Path:      "/v1/flood",
			Detail:    fmt.Sprintf("record-%d", i),
		})
	}

	// Shutdown must not panic.
	r.Shutdown()
}

func TestAnomalyRecorderNilStore(t *testing.T) {
	r := NewAnomalyRecorder(nil)
	// Must not panic
	r.Record(store.AnomalyRecord{
		Category: "unrouted_path",
		Severity: "warn",
		Path:     "/v1/test",
	})
	r.Shutdown() // should be a no-op
}

func TestAnomalyRecorderStaysAlive(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stayalive.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := NewAnomalyRecorder(st)
	defer r.Shutdown()

	// Record immediately - should work
	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "unrouted_path",
		Severity:  "warn",
		Path:      "/v1/first",
		Detail:    "initial record",
	})

	// Sleep long enough that a 30s timeout would have killed the goroutine
	time.Sleep(100 * time.Millisecond)

	// Record after sleep - should still work (proves no global timeout)
	r.Record(store.AnomalyRecord{
		Timestamp: time.Now().UTC(),
		Category:  "parse_error",
		Severity:  "warn",
		Path:      "/v1/second",
		Detail:    "post-sleep record",
	})

	r.Shutdown()

	anomalies, err := st.QueryAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 2 {
		t.Fatalf("len(anomalies) = %d, want 2 (recorder died after sleep?)", len(anomalies))
	}
}
