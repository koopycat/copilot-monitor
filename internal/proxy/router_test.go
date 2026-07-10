package proxy

import "testing"

func TestRoutePath(t *testing.T) {
	r := NewRouter(nil)
	tests := []struct {
		name         string
		path         string
		wantOK       bool
		wantEndpoint Endpoint
		wantUpstream string
		wantCapture  CaptureMode
		wantLocal    bool
	}{
		{name: "ping", path: "/_ping", wantOK: true, wantEndpoint: EndpointPing, wantCapture: CaptureLocal, wantLocal: true},
		{name: "chat", path: "/chat/completions", wantOK: true, wantEndpoint: EndpointChat, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "agent root", path: "/agents", wantOK: true, wantEndpoint: EndpointAgent, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "agent nested", path: "/agents/123", wantOK: true, wantEndpoint: EndpointAgent, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "models", path: "/models", wantOK: true, wantEndpoint: EndpointModels, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureNone},
		{name: "models session", path: "/models/session", wantOK: true, wantEndpoint: EndpointModelsSession, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureNone},
		{name: "responses", path: "/responses", wantOK: true, wantEndpoint: EndpointResponses, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "embeddings", path: "/embeddings", wantOK: true, wantEndpoint: EndpointEmbeddings, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureMetadata},
		{name: "engine completions", path: "/v1/engines/copilot-codex/completions", wantOK: true, wantEndpoint: EndpointCompletions, wantUpstream: GitHubCopilotProxyHost, wantCapture: CaptureUsage},
		{name: "v1 completions", path: "/v1/completions", wantOK: true, wantEndpoint: EndpointCompletions, wantUpstream: GitHubCopilotProxyHost, wantCapture: CaptureUsage},
		{name: "anthropic messages", path: "/v1/messages", wantOK: true, wantEndpoint: EndpointChat, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "anthropic messages nested", path: "/v1/messages/count_tokens", wantOK: true, wantEndpoint: EndpointChat, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureUsage},
		{name: "unknown", path: "/telemetry", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := r.Match(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %t, want %t", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Endpoint != tt.wantEndpoint {
				t.Fatalf("endpoint = %q, want %q", got.Endpoint, tt.wantEndpoint)
			}
			if got.Upstream != tt.wantUpstream {
				t.Fatalf("upstream = %q, want %q", got.Upstream, tt.wantUpstream)
			}
			if got.Capture != tt.wantCapture {
				t.Fatalf("capture = %q, want %q", got.Capture, tt.wantCapture)
			}
			if got.Local != tt.wantLocal {
				t.Fatalf("local = %t, want %t", got.Local, tt.wantLocal)
			}
		})
	}
}

func TestRouter_Match_NilConfig(t *testing.T) {
	r := NewRouter(nil)
	got, ok := r.Match("/_ping")
	if !ok {
		t.Fatal("expected ping route for nil config")
	}
	if got.Endpoint != EndpointPing {
		t.Fatalf("endpoint = %q, want ping", got.Endpoint)
	}
}

func TestRouter_Match_ConfigRoutesOverrideBuiltins(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "custom.example.com", Capture: "none"},
		},
	}
	r := NewRouter(cfg)
	got, ok := r.Match("/chat/completions")
	if !ok {
		t.Fatal("expected route")
	}
	if got.Upstream != "custom.example.com" {
		t.Fatalf("upstream = %q, want custom.example.com", got.Upstream)
	}
	if got.Capture != CaptureNone {
		t.Fatalf("capture = %q, want none", got.Capture)
	}
}

func TestRouter_Match_PrefixRoutes(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1", UpstreamHost: "prefix.example.com", Capture: "none", PrefixMatch: true},
			{Path: "/chat/completions", UpstreamHost: "exact.example.com", Capture: "usage"},
		},
	}
	r := NewRouter(cfg)

	t.Run("exact-match", func(t *testing.T) {
		got, ok := r.Match("/chat/completions")
		if !ok {
			t.Fatal("expected route")
		}
		if got.Upstream != "exact.example.com" {
			t.Fatalf("upstream = %q, want exact.example.com", got.Upstream)
		}
	})

	t.Run("prefix-match", func(t *testing.T) {
		got, ok := r.Match("/v1/completions")
		if !ok {
			t.Fatal("expected route")
		}
		if got.Upstream != "prefix.example.com" {
			t.Fatalf("upstream = %q, want prefix.example.com", got.Upstream)
		}
	})

	t.Run("builtin-fallback", func(t *testing.T) {
		got, ok := r.Match("/_ping")
		if !ok {
			t.Fatal("expected route")
		}
		if got.Capture != CaptureLocal {
			t.Fatalf("capture = %q, want local", got.Capture)
		}
	})
}

func TestRouter_Match_LongestPrefixFirst(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1", UpstreamHost: "short.example.com", Capture: "none", PrefixMatch: true},
			{Path: "/v1/messages", UpstreamHost: "long.example.com", Capture: "usage", PrefixMatch: true},
		},
	}
	r := NewRouter(cfg)
	got, ok := r.Match("/v1/messages/stream")
	if !ok {
		t.Fatal("expected route")
	}
	if got.Upstream != "long.example.com" {
		t.Fatalf("upstream = %q, want long.example.com (longest prefix should win)", got.Upstream)
	}
}

func TestMatchModel_ExactModelMatch(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "openai.example.com", Capture: "usage", Models: []string{"gpt-4o"}},
			{Path: "/v1/chat/completions", UpstreamHost: "anthropic.example.com", Capture: "usage", Models: []string{"claude-*"}},
			{Path: "/v1/chat/completions", UpstreamHost: "default.example.com", Capture: "usage"},
		},
	}
	r := NewRouter(cfg)

	// gpt-4o matches the OpenAI route
	route, ok := r.MatchModel("/v1/chat/completions", "gpt-4o", "")
	if !ok {
		t.Fatal("expected route for gpt-4o")
	}
	if route.Upstream != "openai.example.com" {
		t.Fatalf("upstream = %q, want openai.example.com", route.Upstream)
	}
}

func TestMatchModel_WildcardModelMatch(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "anthropic.example.com", Capture: "usage", Models: []string{"claude-*"}},
		},
	}
	r := NewRouter(cfg)

	for _, model := range []string{"claude-opus", "claude-sonnet-4", "claude-3.5-haiku"} {
		t.Run(model, func(t *testing.T) {
			route, ok := r.MatchModel("/v1/chat/completions", model, "")
			if !ok {
				t.Fatal("expected route")
			}
			if route.Upstream != "anthropic.example.com" {
				t.Fatalf("upstream = %q, want anthropic.example.com", route.Upstream)
			}
		})
	}
}

func TestMatchModel_NoModelsFieldDefaults(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "default.example.com", Capture: "usage"},
		},
	}
	r := NewRouter(cfg)

	for _, model := range []string{"gpt-4o", "claude-opus", ""} {
		t.Run(model, func(t *testing.T) {
			route, ok := r.MatchModel("/v1/chat/completions", model, "")
			if !ok {
				t.Fatal("expected route")
			}
			if route.Upstream != "default.example.com" {
				t.Fatalf("upstream = %q, want default.example.com", route.Upstream)
			}
		})
	}
}

func TestMatchModel_ModelSpecificBeforeDefault(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			// model-specific route first
			{Path: "/v1/chat/completions", UpstreamHost: "openai.example.com", Capture: "usage", Models: []string{"gpt-*"}},
			// default catch-all route last
			{Path: "/v1/chat/completions", UpstreamHost: "default.example.com", Capture: "usage"},
		},
	}
	r := NewRouter(cfg)

	// gpt-4o should match the model-specific route, not the default
	route, ok := r.MatchModel("/v1/chat/completions", "gpt-4o", "")
	if !ok {
		t.Fatal("expected route for gpt-4o")
	}
	if route.Upstream != "openai.example.com" {
		t.Fatalf("upstream = %q, want openai.example.com", route.Upstream)
	}

	// Unmatched model falls to default route
	route, ok = r.MatchModel("/v1/chat/completions", "gemini-pro", "")
	if !ok {
		t.Fatal("expected route for gemini-pro")
	}
	if route.Upstream != "default.example.com" {
		t.Fatalf("upstream = %q, want default.example.com", route.Upstream)
	}
}

func TestMatchModel_EmptyModelFallsThrough(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "openai.example.com", Capture: "usage", Models: []string{"gpt-4o"}},
			{Path: "/v1/chat/completions", UpstreamHost: "default.example.com", Capture: "usage"},
		},
	}
	r := NewRouter(cfg)

	// Empty model should not match model-specific routes; falls to default
	route, ok := r.MatchModel("/v1/chat/completions", "", "")
	if !ok {
		t.Fatal("expected route for empty model")
	}
	if route.Upstream != "default.example.com" {
		t.Fatalf("upstream = %q, want default.example.com", route.Upstream)
	}
}

func TestMatchModel_NoMatchReturnsFalse(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "openai.example.com", Capture: "usage", Models: []string{"gpt-4o"}},
		},
	}
	r := NewRouter(cfg)

	// right path, wrong model -> no configured route matches, no built-in fallback for /v1/chat/completions
	_, ok := r.MatchModel("/v1/chat/completions", "claude-sonnet", "")
	if ok {
		t.Fatal("expected no route for /v1/chat/completions with claude-sonnet")
	}
}

func TestMatchModel_PrefixWithModel(t *testing.T) {
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1", UpstreamHost: "openai.example.com", Capture: "usage", PrefixMatch: true, Models: []string{"gpt-*"}},
			{Path: "/v1", UpstreamHost: "default.example.com", Capture: "usage", PrefixMatch: true},
		},
	}
	r := NewRouter(cfg)

	// gpt-4o through prefix with model filter
	route, ok := r.MatchModel("/v1/chat/completions", "gpt-4o", "")
	if !ok {
		t.Fatal("expected route for gpt-4o")
	}
	if route.Upstream != "openai.example.com" {
		t.Fatalf("upstream = %q, want openai.example.com", route.Upstream)
	}

	// Other model through prefix without model filter
	route, ok = r.MatchModel("/v1/completions", "claude-3", "")
	if !ok {
		t.Fatal("expected route for claude-3")
	}
	if route.Upstream != "default.example.com" {
		t.Fatalf("upstream = %q, want default.example.com", route.Upstream)
	}
}

func TestMatchModel_BuiltinRoutesIgnoreModels(t *testing.T) {
	r := NewRouter(nil)
	// Built-in routes always match regardless of model
	route, ok := r.MatchModel("/chat/completions", "any-model", "")
	if !ok {
		t.Fatal("expected built-in route")
	}
	if route.Upstream != GitHubCopilotAPIHost {
		t.Fatalf("upstream = %q, want %s", route.Upstream, GitHubCopilotAPIHost)
	}
}

func TestRoute_ApplyPathPrefix(t *testing.T) {
	t.Run("prepends", func(t *testing.T) {
		r := Route{UpstreamPathPrefix: "/proxy"}
		path, rawPath := r.ApplyPathPrefix("/api/chat", "/api/chat")
		if path != "/proxy/api/chat" {
			t.Fatalf("path = %q", path)
		}
		if rawPath != "/proxy/api/chat" {
			t.Fatalf("rawPath = %q", rawPath)
		}
	})

	t.Run("passthrough", func(t *testing.T) {
		r := Route{}
		path, rawPath := r.ApplyPathPrefix("/api/chat", "/api/chat")
		if path != "/api/chat" {
			t.Fatalf("path = %q", path)
		}
		if rawPath != "/api/chat" {
			t.Fatalf("rawPath = %q", rawPath)
		}
	})

	t.Run("encoded-rawpath", func(t *testing.T) {
		r := Route{UpstreamPathPrefix: "/v2"}
		path, rawPath := r.ApplyPathPrefix("/api/q%20r", "/api/q%20r")
		if path != "/v2/api/q%20r" {
			t.Fatalf("path = %q", path)
		}
		if rawPath != "/v2/api/q%20r" {
			t.Fatalf("rawPath = %q", rawPath)
		}
	})
}
