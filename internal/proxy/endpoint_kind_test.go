package proxy

import "testing"

func TestClassifyEndpointKind(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		hasModel bool
		hasUsage bool
		want     string
	}{
		{
			name: "model listing is control plane",
			path: "/models", want: EndpointKindControlPlane,
		},
		{
			name: "agent listing is control plane",
			path: "/agents", want: EndpointKindControlPlane,
		},
		{
			name: "prefixed model listing is control plane",
			path: "/v1/models", want: EndpointKindControlPlane,
		},
		{
			name: "copilot-prefixed agent listing is control plane",
			path: "/copilot/agents", want: EndpointKindControlPlane,
		},
		{
			name: "openai-prefixed model listing with trailing slash is control plane",
			path: "/openai/models/", want: EndpointKindControlPlane,
		},
		{
			name: "multiple trailing slashes normalize to known helper",
			path: "/openai/models//", want: EndpointKindControlPlane,
		},
		{
			name: "chat completion with usage is inference",
			path: "/chat/completions", hasUsage: true, want: EndpointKindInference,
		},
		{
			name: "v1 chat completion with model is inference",
			path: "/v1/chat/completions", hasModel: true, want: EndpointKindInference,
		},
		{
			name: "responses endpoint with model is inference",
			path: "/responses", hasModel: true, want: EndpointKindInference,
		},
		{
			name: "unknown path with neither model nor usage is control plane",
			path: "/unknown", want: EndpointKindControlPlane,
		},
		{
			name: "unknown path carrying a model is inference",
			path: "/new-inference", hasModel: true, want: EndpointKindInference,
		},
		{
			name: "unknown path with usage but no model is inference",
			path: "/new-inference", hasUsage: true, want: EndpointKindInference,
		},
		{
			name: "root path with no model or usage is control plane",
			path: "/", want: EndpointKindControlPlane,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyEndpointKind(tt.path, tt.hasModel, tt.hasUsage)
			if got != tt.want {
				t.Errorf("ClassifyEndpointKind(%q, %v, %v) = %q, want %q", tt.path, tt.hasModel, tt.hasUsage, got, tt.want)
			}
		})
	}
}
