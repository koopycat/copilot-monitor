package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"llm-proxy/internal/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
