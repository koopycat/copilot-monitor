package main

import (
	"bytes"
	"log"
	"net/http"
	"testing"
)

func TestSelectUpstream(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		proxyUnknown bool
		wantHost     string
		wantOK       bool
	}{
		{name: "chat", path: "/chat/completions", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "agents", path: "/agents/123", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "models", path: "/models", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "models session", path: "/models/session", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "responses websocket", path: "/responses", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "embeddings", path: "/embeddings", wantHost: githubCopilotAPIHost, wantOK: true},
		{name: "engine completions", path: "/v1/engines/copilot-codex/completions", wantHost: githubCopilotProxyHost, wantOK: true},
		{name: "v1 completions", path: "/v1/completions", wantHost: githubCopilotProxyHost, wantOK: true},
		{name: "unknown rejected", path: "/telemetry", wantOK: false},
		{name: "unknown proxied", path: "/telemetry", proxyUnknown: true, wantHost: githubCopilotAPIHost, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectUpstream(tt.path, tt.proxyUnknown)
			if ok != tt.wantOK {
				t.Fatalf("ok = %t, want %t", ok, tt.wantOK)
			}
			if got.host != tt.wantHost {
				t.Fatalf("host = %q, want %q", got.host, tt.wantHost)
			}
		})
	}
}

func TestRedactHeaders(t *testing.T) {
	headers := http.Header{
		"Authorization":  {"Bearer secret"},
		"Cookie":         {"a=b"},
		"X-Github-Token": {"github-token"},
		"X-Api-Secret":   {"secret"},
		"X-Plain":        {"visible"},
	}

	got := redactHeaders(headers)
	for _, name := range []string{"Authorization", "Cookie", "X-Github-Token", "X-Api-Secret"} {
		values := got[name]
		if len(values) != 1 || values[0] != "<redacted>" {
			t.Fatalf("%s = %#v, want redacted", name, values)
		}
	}
	if got["X-Plain"][0] != "visible" {
		t.Fatalf("X-Plain = %#v, want visible", got["X-Plain"])
	}
}

func TestParseRequestMetadata(t *testing.T) {
	meta := parseRequestMetadata([]byte(`{"model":"gpt-4o","stream":true}`))
	if meta.model != "gpt-4o" {
		t.Fatalf("model = %q", meta.model)
	}
	if !meta.hasStream || !meta.stream {
		t.Fatalf("stream = %t, hasStream = %t", meta.stream, meta.hasStream)
	}
}

func TestSSEObserverDetectsUsageAndModelAcrossSplitLines(t *testing.T) {
	var logs bytes.Buffer
	observer := newSSEObserver(42, log.New(&logs, "", 0))

	observer.observe([]byte("data: {\"mod"))
	observer.observe([]byte("el\":\"gpt-4o\",\"choices\":[],\"usage\":{\"total_tokens\":3}}\n\n"))
	observer.observe([]byte("data: [DONE]\n\n"))

	if !observer.usageSeen {
		t.Fatal("usage was not detected")
	}
	if observer.model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", observer.model)
	}
	if observer.bytes == 0 {
		t.Fatal("bytes were not counted")
	}
	if !bytes.Contains(logs.Bytes(), []byte("usage_detected=true")) {
		t.Fatalf("logs did not include usage detection: %s", logs.String())
	}
}

func TestSSEObserverToleratesMalformedJSON(t *testing.T) {
	var logs bytes.Buffer
	observer := newSSEObserver(1, log.New(&logs, "", 0))

	observer.observe([]byte("data: {broken json}\n"))
	observer.observe([]byte("data: {\"usage\":{\"total_tokens\":1}}\n"))

	if !observer.usageSeen {
		t.Fatal("usage was not detected after malformed event")
	}
}
