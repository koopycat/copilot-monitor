package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestInsertAndStats(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	for _, rec := range []RequestRecord{
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Stream: true, Status: 200, LatencyMS: 50, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Stream: true, Status: 200, LatencyMS: 80, PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
		{Timestamp: now, Endpoint: "completions", Method: "POST", Path: "/v1/completions", UpstreamHost: "copilot-proxy.githubusercontent.com", Model: "copilot", Status: 200, LatencyMS: 20},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := s.Stats(ctx, StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2: %#v", len(stats), stats)
	}
	if stats[0].Model != "gpt-4o" || stats[0].Requests != 2 || stats[0].PromptTokens != 13 || stats[0].CompletionTokens != 7 || stats[0].TotalTokens != 20 {
		t.Fatalf("first row = %#v", stats[0])
	}
}

func TestCompareStatsUsesTwoMonthWindows(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	records := []RequestRecord{
		{Timestamp: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), Endpoint: "chat", Model: "gpt-5-mini", Status: 200, TotalTokens: 100},
		{Timestamp: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), Endpoint: "chat", Model: "gpt-5-mini", Status: 200, TotalTokens: 200},
		{Timestamp: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Endpoint: "chat", Model: "ignored", Status: 200, TotalTokens: 300},
	}
	for _, rec := range records {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	result, err := s.CompareStats(ctx,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Periods) != 2 {
		t.Fatalf("periods = %d, want 2", len(result.Periods))
	}
	if result.Periods[0].TotalTokens != 100 || result.Periods[1].TotalTokens != 200 {
		t.Fatalf("result = %#v", result)
	}
}

func TestStatsFilterSince(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC()
	_ = s.InsertRequest(ctx, RequestRecord{Timestamp: old, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "old", Status: 200})
	_ = s.InsertRequest(ctx, RequestRecord{Timestamp: recent, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "recent", Status: 200})

	stats, err := s.Stats(ctx, StatsFilter{Since: time.Now().UTC().Add(-24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Model != "recent" {
		t.Fatalf("stats = %#v", stats)
	}
}
