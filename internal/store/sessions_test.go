package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestInsertRequestAssignsSessionsIncrementally(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for _, rec := range []RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10, Project: "a"},
		// Joins the first session.
		{Timestamp: base.Add(10 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 20, Project: "a"},
		// Starts a second session.
		{Timestamp: base.Add(50 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 30, Project: "a"},
		// Arrives out of order but is close enough to join the latest session.
		{Timestamp: base.Add(40 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 40, Project: "a"},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	latest, first := sessions[0], sessions[1]
	if latest.RequestCount != 2 || latest.TokenCount != 70 || !latest.EndedAt.Equal(base.Add(50*time.Minute)) {
		t.Fatalf("latest session = %#v", latest)
	}
	if first.RequestCount != 2 || first.TokenCount != 30 {
		t.Fatalf("first session = %#v", first)
	}

	var ids [4]int64
	rows, err := s.db.QueryContext(ctx, "SELECT session_id FROM requests ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i := range ids {
		if !rows.Next() {
			t.Fatal("missing request")
		}
		if err := rows.Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	if ids[0] != ids[1] || ids[2] != ids[3] || ids[0] == ids[2] {
		t.Fatalf("request session IDs = %v", ids)
	}
}

func TestRebuildSessionsReconstructsKnownRequests(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for _, rec := range []RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10, Project: "a"},
		{Timestamp: base.Add(20 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 20, Project: "b"},
		// Exactly 30 minutes after the preceding request begins a new session.
		{Timestamp: base.Add(50 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 30, Project: "b"},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}

	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if got := sessions[1]; got.RequestCount != 2 || got.TokenCount != 30 || got.Project != "<mixed>" {
		t.Fatalf("rebuilt first session = %#v", got)
	}
	if got := sessions[0]; got.RequestCount != 1 || got.TokenCount != 30 || got.Project != "b" {
		t.Fatalf("rebuilt second session = %#v", got)
	}

	var unassigned int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM requests WHERE session_id IS NULL").Scan(&unassigned); err != nil {
		t.Fatal(err)
	}
	if unassigned != 0 {
		t.Fatalf("unassigned requests = %d", unassigned)
	}
}

func TestRebuildSessionsUsesThirtyMinuteGap(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for _, rec := range []RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10, Project: "a"},
		{Timestamp: base.Add(29 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 20, Project: "a"},
		{Timestamp: base.Add(59 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 30, Project: "a"},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2: %#v", len(sessions), sessions)
	}
	if sessions[1].RequestCount != 2 || sessions[1].TokenCount != 30 {
		t.Fatalf("first chronological session = %#v", sessions[1])
	}
	if sessions[0].RequestCount != 1 || sessions[0].TokenCount != 30 {
		t.Fatalf("second chronological session = %#v", sessions[0])
	}
}

func TestRebuildSessionsMarksMixedProjects(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for _, rec := range []RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10, Project: "a"},
		{Timestamp: base.Add(time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 20, Project: "b"},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if sessions[0].Project != "<mixed>" {
		t.Fatalf("project = %q, want mixed", sessions[0].Project)
	}
}

func TestCurrentSessionReturnsLatestSessionWithModels(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Now().UTC().Add(-10 * time.Minute)
	for _, rec := range []RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-5-mini", Status: 200, PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, Project: "a"},
		{Timestamp: base.Add(time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "claude-sonnet-4.6", Status: 200, PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300, Project: "a"},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}

	current, err := s.CurrentSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil {
		t.Fatal("current session is nil")
	}
	if !current.Active || current.Status != "active" {
		t.Fatalf("status = %q active = %v", current.Status, current.Active)
	}
	if current.RequestCount != 2 || current.TokenCount != 450 || current.Project != "a" {
		t.Fatalf("current = %#v", current)
	}
	if len(current.Models) != 2 {
		t.Fatalf("models = %#v", current.Models)
	}
	if current.Models[0].Model != "claude-sonnet-4.6" || current.Models[0].TotalTokens != 300 {
		t.Fatalf("models = %#v", current.Models)
	}
}

func TestCurrentSessionReturnsIdleForOldLatestSession(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	old := time.Now().UTC().Add(-2 * time.Hour)
	if err := s.InsertRequest(ctx, RequestRecord{Timestamp: old, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-5-mini", Status: 200, TotalTokens: 10}); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}

	current, err := s.CurrentSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil {
		t.Fatal("current session is nil")
	}
	if current.Active || current.Status != "idle" {
		t.Fatalf("status = %q active = %v", current.Status, current.Active)
	}
}

func TestCurrentSessionEmptyDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	current, err := s.CurrentSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil", current)
	}
}

func TestSessionModelsBatch(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for _, rec := range []RequestRecord{
		// First session: one request.
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "gpt-4o", Status: 200, LatencyMS: 10, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		// Second session: two models.
		{Timestamp: base.Add(time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "gpt-4o", Status: 200, LatencyMS: 20, PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
		{Timestamp: base.Add(time.Hour + time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "claude-sonnet", Status: 200, LatencyMS: 40, PromptTokens: 25, CompletionTokens: 25, TotalTokens: 50},
		// Third session: two gpt-5 requests must be aggregated into one row.
		{Timestamp: base.Add(2 * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "gpt-5", Status: 200, LatencyMS: 30, PromptTokens: 15, CompletionTokens: 10, TotalTokens: 25},
		{Timestamp: base.Add(2*time.Hour + time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "gpt-5", Status: 200, LatencyMS: 50, PromptTokens: 10, CompletionTokens: 15, TotalTokens: 25},
		{Timestamp: base.Add(2*time.Hour + 2*time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Model: "gemini-pro", Status: 200, LatencyMS: 60, PromptTokens: 30, CompletionTokens: 10, TotalTokens: 40},
	} {
		if err := s.InsertRequest(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := s.Sessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("sessions = %#v, want three", sessions)
	}
	idsByStart := make(map[string]int64, len(sessions))
	for _, session := range sessions {
		idsByStart[session.StartedAt.Format(time.RFC3339Nano)] = session.ID
	}
	ids := []int64{
		idsByStart[base.Format(time.RFC3339Nano)],
		idsByStart[base.Add(time.Hour).Format(time.RFC3339Nano)],
		idsByStart[base.Add(2*time.Hour).Format(time.RFC3339Nano)],
	}

	models, err := s.SessionModelsBatch(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64][]ModelStats{
		ids[0]: {{Model: "gpt-4o", Endpoint: "chat", UpstreamHost: "api.example.test", Requests: 1, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, AvgLatencyMS: 10}},
		ids[1]: {
			{Model: "claude-sonnet", Endpoint: "chat", UpstreamHost: "api.example.test", Requests: 1, PromptTokens: 25, CompletionTokens: 25, TotalTokens: 50, AvgLatencyMS: 40},
			{Model: "gpt-4o", Endpoint: "chat", UpstreamHost: "api.example.test", Requests: 1, PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, AvgLatencyMS: 20},
		},
		ids[2]: {
			{Model: "gpt-5", Endpoint: "chat", UpstreamHost: "api.example.test", Requests: 2, PromptTokens: 25, CompletionTokens: 25, TotalTokens: 50, AvgLatencyMS: 40},
			{Model: "gemini-pro", Endpoint: "chat", UpstreamHost: "api.example.test", Requests: 1, PromptTokens: 30, CompletionTokens: 10, TotalTokens: 40, AvgLatencyMS: 60},
		},
	}
	if len(models) != len(want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
	for id, expected := range want {
		got := models[id]
		if len(got) != len(expected) {
			t.Fatalf("models[%d] = %#v, want %#v", id, got, expected)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("models[%d][%d] = %#v, want %#v", id, i, got[i], expected[i])
			}
		}
	}

	for _, sessionIDs := range [][]int64{{}, nil} {
		models, err := s.SessionModelsBatch(ctx, sessionIDs)
		if err != nil {
			t.Fatal(err)
		}
		if models == nil || len(models) != 0 {
			t.Fatalf("models for empty IDs = %#v, want empty map", models)
		}
	}
}

func TestCountSessions(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	count, err := s.CountSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("empty store session count = %d, want 0", count)
	}

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := s.InsertRequest(ctx, RequestRecord{Timestamp: base.Add(time.Duration(i) * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
	count, err = s.CountSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("session count = %d, want 3", count)
	}
}

func TestDistinctProjects(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	projects, err := s.DistinctProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("empty store projects = %#v, want non-nil empty slice", projects)
	}

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i, project := range []string{"alpha", "beta", "alpha"} {
		if err := s.InsertRequest(ctx, RequestRecord{Timestamp: base.Add(time.Duration(i) * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.example.test", Status: 200, Project: project}); err != nil {
			t.Fatal(err)
		}
	}
	projects, err = s.DistinctProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0] != "alpha" || projects[1] != "beta" {
		t.Fatalf("projects = %#v, want [alpha beta]", projects)
	}
}

func TestSessionModelsBatchEmptyStore(t *testing.T) {
	var s *Store
	if _, err := s.SessionModelsBatch(context.Background(), []int64{1}); err == nil {
		t.Fatal("SessionModelsBatch(nil store) error = nil, want error")
	}
}

func TestSessionsFilterSinceAndLimit(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := s.InsertRequest(ctx, RequestRecord{Timestamp: base.Add(time.Duration(i) * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RebuildSessions(ctx, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, SessionFilter{Since: base.Add(30 * time.Minute), Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if !sessions[0].StartedAt.Equal(base.Add(2 * time.Hour)) {
		t.Fatalf("started = %s", sessions[0].StartedAt)
	}
}
