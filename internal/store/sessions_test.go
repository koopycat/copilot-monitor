package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

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
