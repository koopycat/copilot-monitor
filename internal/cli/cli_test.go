package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"llm-proxy/internal/catalog"
	costcalc "llm-proxy/internal/cost"
	"llm-proxy/internal/store"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "llm-proxy") {
		t.Fatalf("unexpected version output: %s", stdout.String())
	}
}

func TestCompressionMode(t *testing.T) {
	if got := compressionMode(false); got != "fail-open" {
		t.Fatalf("compressionMode(false) = %q", got)
	}
	if got := compressionMode(true); got != "required" {
		t.Fatalf("compressionMode(true) = %q", got)
	}
}

func TestStatsCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	_ = st.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "--db", dbPath, "--since", "all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "gpt-4o") || !strings.Contains(out, "15") {
		t.Fatalf("unexpected stats output:\n%s", out)
	}
}

func TestCostCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-5-mini", Status: 200, PromptTokens: 1_000_000, CachedInputTokens: 250_000, CompletionTokens: 500_000, TotalTokens: 1_500_000})
	_ = st.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"cost", "--db", dbPath, "--since", "all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Estimated equivalent LLM API list-price cost", "gpt-5-mini", "openai", "1.193750", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestStatsCommandJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	_ = st.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "--db", dbPath, "--since", "all", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "gpt-4o") || !strings.Contains(out, "\"prompt_tokens\"") {
		t.Fatalf("unexpected JSON output:\n%s", out)
	}
}

func TestTodayCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	_ = st.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"today", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage since") || !strings.Contains(out, "gpt-4o") {
		t.Fatalf("unexpected today output:\n%s", out)
	}
}

func TestSessionsCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Hour)
	for _, rec := range []store.RequestRecord{
		{Timestamp: base, Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 10, Project: "p"},
		{Timestamp: base.Add(time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, TotalTokens: 20, Project: "p"},
	} {
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sessions", "--db", dbPath, "--since", "all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "REQUESTS") || !strings.Contains(out, "p") || !strings.Contains(out, "30") {
		t.Fatalf("unexpected sessions output:\n%s", out)
	}
}

func TestLiveCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, rec := range []store.RequestRecord{
		{Timestamp: now.Add(-5 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "claude-3.5-sonnet", Status: 200, PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, Project: "p"},
		{Timestamp: now.Add(-2 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "claude-3.5-sonnet", Status: 200, PromptTokens: 800, CompletionTokens: 400, TotalTokens: 1200, Project: "p"},
	} {
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"live", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Live session",
		"● active",
		"claude-3.5-sonnet",
		"p",
		"2",     // request count
		"2,700", // total tokens
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("live output missing %q:\n%s", want, out)
		}
	}
}

func TestLiveCommandEmptyDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"live", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No sessions captured yet") {
		t.Fatalf("unexpected live output:\n%s", stdout.String())
	}
}

func TestRenderLiveCompact(t *testing.T) {
	now := time.Now().UTC()
	cat, err := catalog.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	current := &store.CurrentSession{
		StartedAt:     now.Add(-5 * time.Minute),
		LastRequestAt: now.Add(-1 * time.Minute),
		Project:       "p",
		RequestCount:  7,
		TokenCount:    12345,
		Status:        "active",
		Active:        true,
		Models: []store.ModelStats{
			{Model: "claude-3.5-sonnet", Endpoint: "chat/completions", Requests: 4, TotalTokens: 6000},
			{Model: "gpt-4.1", Endpoint: "chat/completions", Requests: 2, TotalTokens: 4000},
			{Model: "o3", Endpoint: "chat/completions", Requests: 1, TotalTokens: 2345},
		},
	}
	costResult := costcalc.Calculate(current.Models, cat)
	out := renderLiveCompact(current, costResult)
	for _, want := range []string{"● active", "7 req", "12,345 tok", "project p", "claude-3.5-sonnet 4", "gpt-4.1 2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("compact render missing %q:\n%s", want, out)
		}
	}
	// Empty current should produce a placeholder
	emptyOut := renderLiveCompact(nil, costcalc.Total{})
	if !strings.Contains(emptyOut, "no sessions") {
		t.Fatalf("empty render should mention no sessions, got: %q", emptyOut)
	}
}

func TestRenderLiveModelOverview(t *testing.T) {
	now := time.Now().UTC()
	cat, err := catalog.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	current := &store.CurrentSession{
		StartedAt:     now.Add(-5 * time.Minute),
		LastRequestAt: now.Add(-1 * time.Minute),
		Project:       "p",
		RequestCount:  6,
		TokenCount:    9000,
		Status:        "active",
		Active:        true,
		Models: []store.ModelStats{
			{Model: "claude-3.5-sonnet", Endpoint: "chat/completions", Requests: 4, PromptTokens: 1000, CachedInputTokens: 100, CompletionTokens: 500},
			{Model: "claude-3.5-sonnet", Endpoint: "responses", Requests: 1, PromptTokens: 500, CachedInputTokens: 0, CompletionTokens: 200},
			{Model: "gpt-4.1", Endpoint: "chat/completions", Requests: 1, PromptTokens: 300, CachedInputTokens: 200, CompletionTokens: 100},
		},
	}
	costResult := costcalc.Calculate(current.Models, cat)

	var buf bytes.Buffer
	renderLive(&buf, current, costResult)
	out := buf.String()
	// Model overview header
	for _, want := range []string{
		"MODEL",
		"REQUESTS",
		"CACHE HIT",
		"COST",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("live overview missing %q:\n%s", want, out)
		}
	}
	// claude-3.5-sonnet should be aggregated (4+1=5 req, not 4+1 separately)
	// and should appear exactly once in the model section
	if got := strings.Count(out, "claude-3.5-sonnet"); got != 1 {
		t.Fatalf("claude-3.5-sonnet should appear once (aggregated), got %d:\n%s", got, out)
	}
	// TOTAL row should be present
	if !strings.Contains(out, "TOTAL ") {
		t.Fatalf("live overview missing TOTAL row:\n%s", out)
	}
	// Cache hit percent for claude-3.5-sonnet: 100/(1000+100+500+0) = 100/1600 ≈ 6%
	if !strings.Contains(out, "6%") {
		t.Fatalf("live overview missing 6%% cache hit for claude-3.5-sonnet:\n%s", out)
	}
}

func TestLiveCommandJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.InsertRequest(context.Background(), store.RequestRecord{
		Timestamp: now.Add(-time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4.1", Status: 200, PromptTokens: 500, CompletionTokens: 250, TotalTokens: 750,
	}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"live", "--db", dbPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"status": "active"`, `"request_count": 1`, `"gpt-4.1"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("live json missing %q:\n%s", want, out)
		}
	}
}

func TestCostCommandFallbackNote(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "unknown-model", Status: 200, PromptTokens: 1_000_000, CompletionTokens: 1_000_000, TotalTokens: 2_000_000})
	_ = st.Close()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"cost", "--db", dbPath, "--since", "all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "fallback pricing used") && !strings.Contains(out, "fallback pricing") {
		t.Fatalf("expected fallback note:\n%s", out)
	}
}

func TestParseSinceDays(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	got, err := parseSince("7d", now)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(-7 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestExportCommandCSV(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CachedInputTokens: 5, CompletionTokens: 2, TotalTokens: 12, Project: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "agent", Method: "POST", Path: "/agents", UpstreamHost: "api.githubcopilot.com", Model: "<unknown>", Status: 200}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "--db", dbPath, "--since", "all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + at least 1 row, got:\n%s", out)
	}
	header := lines[0]
	for _, want := range []string{"ts", "endpoint", "model", "prompt_tokens", "cached_input_tokens", "total_tokens", "project"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q: %s", want, header)
		}
	}
	if !strings.Contains(out, "gpt-4o") {
		t.Fatalf("expected row referencing gpt-4o, got:\n%s", out)
	}
	if !strings.Contains(out, ",demo") {
		t.Fatalf("expected demo project in a row, got:\n%s", out)
	}
	if strings.Contains(out, "<unknown>,agent,200") {
		t.Fatalf("row with empty model should be excluded, got:\n%s", out)
	}
}

func TestExportCommandInvalidDB(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "--db", "/nonexistent/dir/store.db"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit code, got 0")
	}
}
