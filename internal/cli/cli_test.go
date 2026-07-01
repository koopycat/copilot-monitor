package cli

import (
	"bytes"
	"strings"
	"testing"
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
