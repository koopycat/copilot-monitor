package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"copilot-monitoring/internal/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogCaching(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	first, err := s.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if first.Models == nil || second.Models == nil {
		t.Fatal("catalog models are nil")
	}
	if reflect.ValueOf(first.Models).Pointer() != reflect.ValueOf(second.Models).Pointer() {
		t.Fatal("Catalog returned different cached catalog instances")
	}
}

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

func TestGetPolicyDefault(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	p, err := s.GetPolicy(ctx)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, policy.AllowAll, p.Mode)
	assert.Empty(t, p.Models)
}

func TestSetPolicyAndGet(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	p := &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o", "claude-*"}}
	err = s.SetPolicy(ctx, p)
	require.NoError(t, err)

	got, err := s.GetPolicy(ctx)
	require.NoError(t, err)
	assert.Equal(t, policy.Blocklist, got.Mode)
	assert.ElementsMatch(t, []string{"gpt-4o", "claude-*"}, got.Models)
}

func TestDistinctModels(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	recs := []RequestRecord{
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.openai.com", Model: "gpt-4o", Status: 200},
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.openai.com", Model: "claude-3.5-sonnet", Status: 200},
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.openai.com", Model: "gpt-4o", Status: 200},
	}
	for _, rec := range recs {
		err = s.InsertRequest(ctx, rec)
		require.NoError(t, err)
	}

	models, err := s.DistinctModels(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"claude-3.5-sonnet", "gpt-4o"}, models)
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

func TestWriteAndQueryAnomalies(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Write anomalies
	if err := s.WriteAnomaly(ctx, AnomalyRecord{
		Timestamp: now, Category: "unrouted_path", Severity: "warn",
		Path: "/v1/new-endpoint", Method: "POST", Detail: "no route matched",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAnomaly(ctx, AnomalyRecord{
		Timestamp: now.Add(-1 * time.Hour), Category: "parse_error", Severity: "warn",
		Path: "/chat/completions", Detail: "malformed JSON in SSE",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteAnomaly(ctx, AnomalyRecord{
		Timestamp: now.Add(-2 * time.Hour), Category: "auth_missing", Severity: "error",
		Path: "/chat/completions", Detail: "no Authorization header",
	}); err != nil {
		t.Fatal(err)
	}

	// Query all
	all, err := s.QueryAnomalies(ctx, AnomalyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}

	// Query by category
	cat, err := s.QueryAnomalies(ctx, AnomalyFilter{Category: "unrouted_path"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 1 || cat[0].Category != "unrouted_path" {
		t.Fatalf("cat = %#v", cat)
	}

	// Query by severity
	sev, err := s.QueryAnomalies(ctx, AnomalyFilter{Severity: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sev) != 1 || sev[0].Category != "auth_missing" {
		t.Fatalf("sev = %#v", sev)
	}

	// Query by time
	recent, err := s.QueryAnomalies(ctx, AnomalyFilter{Since: now.Add(-30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].Category != "unrouted_path" {
		t.Fatalf("recent = %#v", recent)
	}
}
