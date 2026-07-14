package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRetentionStateLifecycle(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SetRetentionDays(90)
	status := s.RetentionStatus()
	if status.RetentionDays != 90 {
		t.Fatalf("retention days = %d, want 90", status.RetentionDays)
	}

	prunedAt := time.Now().UTC()
	s.RecordPrune(prunedAt, 42)
	status = s.RetentionStatus()
	if !status.LastPruneAt.Equal(prunedAt) || status.PrunedCount != 42 {
		t.Fatalf("retention status = %#v, want prune at %s with count 42", status, prunedAt)
	}

	status.RetentionDays = 1
	status.LastPruneAt = time.Time{}
	status.PrunedCount = 0
	unchanged := s.RetentionStatus()
	if unchanged.RetentionDays != 90 || !unchanged.LastPruneAt.Equal(prunedAt) || unchanged.PrunedCount != 42 {
		t.Fatalf("retention status was mutated through returned copy: %#v", unchanged)
	}

	s.SetRetentionDays(0)
	if got := s.RetentionStatus().RetentionDays; got != 0 {
		t.Fatalf("retention days after reset = %d, want 0", got)
	}
}

func TestRetentionRowCount(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if err := s.InsertRequest(ctx, RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat",
			UpstreamHost: "api.example.test", Status: 200,
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteAnomaly(ctx, AnomalyRecord{Timestamp: base.Add(time.Duration(i) * time.Hour), Category: "parse_error", Severity: "warn"}); err != nil {
			t.Fatal(err)
		}
	}

	count, err := s.RetentionRowCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 { // two requests, two sessions, and two anomalies
		t.Fatalf("retention row count = %d, want 6", count)
	}

	var nilStore *Store
	if _, err := nilStore.RetentionRowCount(ctx); err == nil {
		t.Fatal("RetentionRowCount(nil store) error = nil, want error")
	}
}

func TestPruneAnomaliesZeroCutoff(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.WriteAnomaly(ctx, AnomalyRecord{Timestamp: time.Now().UTC(), Category: "parse_error", Severity: "warn"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.PruneAnomalies(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("deleted anomalies = %d, want 0", deleted)
	}
	anomalies, err := s.QueryAnomalies(ctx, AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("remaining anomalies = %#v, want one", anomalies)
	}
}

func TestPruneRequestsRetainsSessionCrossingCutoff(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	cutoff := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for _, timestamp := range []time.Time{
		cutoff.Add(-3 * time.Hour), cutoff.Add(-179 * time.Minute), // entirely old session
		cutoff.Add(-time.Minute), cutoff.Add(time.Minute), // crosses cutoff
	} {
		if err := s.InsertRequest(ctx, RequestRecord{
			Timestamp: timestamp, Endpoint: "chat", Method: "POST", Path: "/chat",
			UpstreamHost: "api.example.test", Status: 200,
		}); err != nil {
			t.Fatal(err)
		}
	}

	counts, err := s.PrunableCounts(ctx, cutoff, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Requests != 2 || counts.Sessions != 1 {
		t.Fatalf("prunable counts = %#v, want 2 requests and 1 session", counts)
	}
	deleted, err := s.PruneRequests(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	var requests, sessions int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests").Scan(&requests); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || sessions != 1 {
		t.Fatalf("remaining = %d requests, %d sessions; want 2 and 1", requests, sessions)
	}
}

func TestPruneRequestsBatchesAndPrunableCountsDoNotDelete(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().Add(-48 * time.Hour)
	for i := 0; i < pruneBatchSize+1; i++ {
		if err := s.InsertRequest(ctx, RequestRecord{
			Timestamp: old.Add(time.Duration(i) * time.Millisecond), Endpoint: "chat", Method: "POST", Path: "/chat",
			UpstreamHost: "api.example.test", Status: 200,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.WriteAnomaly(ctx, AnomalyRecord{Timestamp: old, Category: "parse_error", Severity: "warn"}); err != nil {
		t.Fatal(err)
	}

	before := time.Now().UTC().Add(-24 * time.Hour)
	counts, err := s.PrunableCounts(ctx, before, before)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Requests != pruneBatchSize+1 || counts.Anomalies != 1 {
		t.Fatalf("dry-run counts = %#v", counts)
	}
	var beforeDelete int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests").Scan(&beforeDelete); err != nil {
		t.Fatal(err)
	}
	if beforeDelete != pruneBatchSize+1 {
		t.Fatalf("dry-run modified requests: got %d", beforeDelete)
	}

	deleted, err := s.PruneRequests(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != pruneBatchSize+1 {
		t.Fatalf("deleted = %d, want %d", deleted, pruneBatchSize+1)
	}
	anomalies, err := s.PruneAnomalies(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if anomalies != 1 {
		t.Fatalf("deleted anomalies = %d, want 1", anomalies)
	}
}
