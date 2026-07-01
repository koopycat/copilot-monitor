package proxy

import "testing"

func TestRoutePath(t *testing.T) {
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
		{name: "responses", path: "/responses", wantOK: true, wantEndpoint: EndpointResponses, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureTunnel},
		{name: "embeddings", path: "/embeddings", wantOK: true, wantEndpoint: EndpointEmbeddings, wantUpstream: GitHubCopilotAPIHost, wantCapture: CaptureMetadata},
		{name: "engine completions", path: "/v1/engines/copilot-codex/completions", wantOK: true, wantEndpoint: EndpointCompletions, wantUpstream: GitHubCopilotProxyHost, wantCapture: CaptureUsage},
		{name: "v1 completions", path: "/v1/completions", wantOK: true, wantEndpoint: EndpointCompletions, wantUpstream: GitHubCopilotProxyHost, wantCapture: CaptureUsage},
		{name: "unknown", path: "/telemetry", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RoutePath(tt.path)
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
