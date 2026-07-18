package proxy

import "strings"

const (
	EndpointKindInference    = "inference"
	EndpointKindControlPlane = "control_plane"
)

// ClassifyEndpointKind classifies a proxied endpoint as inference
// (model-generation traffic) or control_plane (helper / discovery / metadata
// traffic that carries no model and no usage).
//
// It normalizes a single leading /v1/, /copilot/, or /openai/ prefix and any
// trailing slash. Known helper listings (/models, /agents) are always
// control_plane. Otherwise, a request with no model and no captured usage is
// control_plane; everything else is inference.
func ClassifyEndpointKind(path string, hasModel bool, hasUsage bool) string {
	normalized := normalizeEndpointPath(path)
	switch normalized {
	case "/models", "/agents":
		return EndpointKindControlPlane
	}
	if !hasModel && !hasUsage {
		return EndpointKindControlPlane
	}
	return EndpointKindInference
}

// normalizeEndpointPath strips a single leading provider-version prefix and
// any trailing slash so that /v1/models, /copilot/models/, /openai/models,
// and /models all normalize to /models.
func normalizeEndpointPath(path string) string {
	p := path
	for _, prefix := range []string{"/v1/", "/copilot/", "/openai/"} {
		if strings.HasPrefix(p, prefix) {
			p = p[len(prefix)-1:]
			break
		}
	}
	if p == "" {
		p = "/"
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		p = "/"
	}
	return p
}
