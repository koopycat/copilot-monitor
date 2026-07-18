package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"copilot-monitoring/internal/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
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

func TestEndpointKindMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Create a database with the pre-endpoint_kind schema.
	legacyDB, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacyDB.ExecContext(ctx, `
CREATE TABLE requests (
  id INTEGER PRIMARY KEY,
  ts TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  upstream_host TEXT NOT NULL,
  model TEXT,
  stream INTEGER NOT NULL,
  status INTEGER NOT NULL,
  error TEXT,
  latency_ms INTEGER NOT NULL,
  prompt_tokens INTEGER,
  cached_input_tokens INTEGER,
  cache_write_tokens INTEGER,
  completion_tokens INTEGER,
  total_tokens INTEGER,
  project TEXT,
  not_billed INTEGER NOT NULL DEFAULT 0,
  provider TEXT NOT NULL DEFAULT '',
  session_id INTEGER,
  usage_missing INTEGER NOT NULL DEFAULT 0,
  headroom_proxied INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE sessions (id INTEGER PRIMARY KEY, started_at TEXT, ended_at TEXT, project TEXT, request_count INTEGER, token_count INTEGER);
CREATE TABLE policies (id INTEGER PRIMARY KEY CHECK (id = 1), mode TEXT NOT NULL DEFAULT 'allow_all', models_json TEXT NOT NULL DEFAULT '[]');
CREATE TABLE anomalies (id INTEGER PRIMARY KEY, ts TEXT, category TEXT, severity TEXT, request_id INTEGER, path TEXT, method TEXT, endpoint TEXT, model TEXT, upstream TEXT, status INTEGER, detail TEXT, json_detail TEXT);`)
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = legacyDB.ExecContext(ctx, `
INSERT INTO requests (ts, endpoint, method, path, upstream_host, model, stream, status, latency_ms, prompt_tokens, total_tokens, usage_missing, project)
VALUES
  (?, 'chat', 'POST', '/chat/completions', 'api.example.com', 'gpt-4o', 0, 200, 10, 5, 7, 0, 'proj'),
  (?, 'models', 'GET', '/models', 'api.example.com', NULL, 0, 200, 1, 0, 0, 1, NULL),
  (?, 'agents', 'GET', '/agents', 'api.example.com', '', 0, 200, 1, 0, 0, 1, NULL),
  (?, 'chat', 'POST', '/chat/completions', 'api.example.com', 'gpt-4o', 0, 200, 0, 0, 0, 0, 'proj')`,
		now, now, now, now)
	require.NoError(t, err)
	require.NoError(t, legacyDB.Close())

	// Re-open with the current store implementation. This should add the column
	// and backfill it.
	s, err := Open(path)
	require.NoError(t, err)
	defer s.Close()

	// Verify endpoint_kind is populated for all rows.
	rows, err := s.db.QueryContext(ctx, "SELECT endpoint, endpoint_kind FROM requests ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()

	expected := []struct {
		endpoint     string
		endpointKind string
	}{
		{"chat", EndpointKindInference},
		{"models", EndpointKindControlPlane},
		{"agents", EndpointKindControlPlane},
		{"chat", EndpointKindInference}, // model present but no tokens -> still inference
	}
	count := 0
	for rows.Next() {
		var endpoint, endpointKind string
		require.NoError(t, rows.Scan(&endpoint, &endpointKind))
		require.NotEmpty(t, endpointKind, "row %d should have endpoint_kind backfilled", count)
		require.Equal(t, expected[count].endpoint, endpoint)
		require.Equal(t, expected[count].endpointKind, endpointKind)
		count++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(expected), count)

	// Stats should exclude the control-plane rows.
	stats, err := s.Stats(ctx, StatsFilter{})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "gpt-4o", stats[0].Model)
	require.Equal(t, 2, stats[0].Requests, "both inference chat rows should be aggregated")
	require.Equal(t, 7, stats[0].TotalTokens)
}

func TestEndpointKindFilter(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()

	now := time.Now().UTC()
	recs := []RequestRecord{
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.example.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, EndpointKind: EndpointKindInference},
		{Timestamp: now, Endpoint: "models", Method: "GET", Path: "/models", UpstreamHost: "api.example.com", Status: 200, EndpointKind: EndpointKindControlPlane},
		{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.example.com", Model: "gpt-4o", Status: 200, EndpointKind: EndpointKindInference, UsageMissing: true},
	}
	for _, rec := range recs {
		require.NoError(t, s.InsertRequest(ctx, rec))
	}

	stats, err := s.Stats(ctx, StatsFilter{})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, 2, stats[0].Requests, "inference rows with and without usage_missing should be included")

	timeline, err := s.Timeline(ctx, StatsFilter{}, "day")
	require.NoError(t, err)
	require.Len(t, timeline, 1)

	export, err := s.ExportRequests(ctx, time.Time{}, time.Time{}, "", "", "")
	require.NoError(t, err)
	require.Len(t, export, 3, "export should include all rows regardless of endpoint_kind")
}
