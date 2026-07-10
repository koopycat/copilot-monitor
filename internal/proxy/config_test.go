package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.json")
	content := `{
		"routes": [
			{"path": "/chat/completions", "upstream_host": "api.example.com", "capture": "usage"},
			{"path": "/models", "upstream_host": "models.example.com", "capture": "none"}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(cfg.Routes))
	}
	if cfg.Routes[0].Path != "/chat/completions" {
		t.Fatalf("route[0].path = %q", cfg.Routes[0].Path)
	}
	if cfg.Routes[0].UpstreamHost != "api.example.com" {
		t.Fatalf("route[0].upstream_host = %q", cfg.Routes[0].UpstreamHost)
	}
	if cfg.Routes[0].Capture != "usage" {
		t.Fatalf("route[0].capture = %q", cfg.Routes[0].Capture)
	}
	if cfg.Routes[1].Path != "/models" {
		t.Fatalf("route[1].path = %q", cfg.Routes[1].Path)
	}
}

func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for empty path")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nonexistent.json") {
		t.Fatalf("error does not mention path: %v", err)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse routes config") {
		t.Fatalf("error does not mention parse: %v", err)
	}
}

func TestValidate_MissingPath(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{UpstreamHost: "example.com", Capture: "none"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_MissingUpstreamHost(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat", Capture: "usage"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "upstream_host is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_BadCaptureMode(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat", UpstreamHost: "example.com", Capture: "invalid"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown capture mode") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_PathWithoutLeadingSlash(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "chat/completions", UpstreamHost: "example.com", Capture: "none"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must start with /") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_LabelSet(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Label: "custom-chat", Path: "/chat/completions", UpstreamHost: "example.com", Capture: "usage"},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_LabelEmptyAfterTrim(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Label: "   ", Path: "/chat", UpstreamHost: "example.com", Capture: "none"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "label must not be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_ModelsSet(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "example.com", Capture: "usage", Models: []string{"gpt-4o", "claude-*"}},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Routes[0].Models) != 2 {
		t.Fatalf("models = %#v, want 2 entries", cfg.Routes[0].Models)
	}
	if cfg.Routes[0].Models[0] != "gpt-4o" {
		t.Fatalf("models[0] = %q", cfg.Routes[0].Models[0])
	}
}

func TestValidate_ModelsEmptyString(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "example.com", Capture: "usage", Models: []string{""}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "models[0] must not be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_ModelsWhitespaceOnlyString(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "example.com", Capture: "usage", Models: []string{"   "}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "models[0] must not be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_ModelsEmptySlice(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "example.com", Capture: "usage", Models: []string{}},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_TrimsWhitespaceAndNormalizes(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "  /chat/completions  ", UpstreamHost: " example.com ", Capture: "usage", UpstreamPathPrefix: "/proxy/"},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[0].Path != "/chat/completions" {
		t.Fatalf("path not trimmed: %q", cfg.Routes[0].Path)
	}
	if cfg.Routes[0].UpstreamHost != "example.com" {
		t.Fatalf("upstream_host not trimmed: %q", cfg.Routes[0].UpstreamHost)
	}
	if cfg.Routes[0].UpstreamPathPrefix != "/proxy" {
		t.Fatalf("upstream_path_prefix not normalized: %q", cfg.Routes[0].UpstreamPathPrefix)
	}
}
