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
	err = st.InsertRequest(context.Background(), store.RequestRecord{Timestamp: time.Now().UTC(), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: 200, PromptTokens: 1_000_000, CompletionTokens: 500_000, TotalTokens: 1_500_000})
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
	for _, want := range []string{"Estimated equivalent provider list-price cost", "gpt-4o", "openai", "7.500000", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
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
	if !strings.Contains(out, "fallback pricing used") {
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
