package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionZsh(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "zsh"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"_copilot-monitor",
		"compdef",
		"run:Start",
		"serve:Start",
		"stats:Print",
		"cost:Print",
		"today:Print",
		"sessions:Print",
		"live:Print",
		"export:Export",
		"init:Create",
		"validate:Validate",
		"inspect:Show",
		"version:Print",
		"help:Show",
		"completion:Generate",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("completion zsh output missing %q", want)
		}
	}
	if stderr.Len() > 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
}

func TestCompletionBash(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "bash"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for unsupported shell")
	}
	if !strings.Contains(stderr.String(), `unsupported shell "bash"`) {
		t.Fatalf("expected unsupported shell error, got: %s", stderr.String())
	}
	if stdout.Len() > 0 {
		t.Fatalf("expected no stdout output, got: %s", stdout.String())
	}
}

func TestCompletionNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for missing shell argument")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage error, got: %s", stderr.String())
	}
}
