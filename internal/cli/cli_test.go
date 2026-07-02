package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestConfigureVSCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"configure-vscode", "--addr", "127.0.0.1:9999"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		`"debug.overrideProxyUrl": "http://127.0.0.1:9999"`,
		`"debug.overrideCapiUrl": "http://127.0.0.1:9999"`,
		`"authProvider": "github"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestConfigureVSCodeWithBarePort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"configure-vscode", "--addr", ":7733"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "http://127.0.0.1:7733") {
		t.Fatalf("expected normalized localhost URL, got:\n%s", stdout.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "copilot-monitor") {
		t.Fatalf("unexpected version output: %s", stdout.String())
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
	for _, want := range []string{"Estimated equivalent GitHub Copilot AI-credit list-price cost", "gpt-5-mini", "openai", "1.193750", "TOTAL"} {
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
