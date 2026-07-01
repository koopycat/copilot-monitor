package proxy

import "strings"

const (
	GitHubCopilotAPIHost   = "api.githubcopilot.com"
	GitHubCopilotProxyHost = "copilot-proxy.githubusercontent.com"
)

type CaptureMode string

const (
	CaptureNone     CaptureMode = "none"
	CaptureMetadata CaptureMode = "metadata"
	CaptureUsage    CaptureMode = "usage"
	CaptureTunnel   CaptureMode = "tunnel"
	CaptureLocal    CaptureMode = "local"
)

type Endpoint string

const (
	EndpointChat          Endpoint = "chat"
	EndpointAgent         Endpoint = "agent"
	EndpointModels        Endpoint = "models"
	EndpointModelsSession Endpoint = "models-session"
	EndpointResponses     Endpoint = "responses-websocket"
	EndpointEmbeddings    Endpoint = "embeddings"
	EndpointCompletions   Endpoint = "completions"
	EndpointPing          Endpoint = "ping"
)

type Route struct {
	Endpoint Endpoint
	Upstream string
	Capture  CaptureMode
	Local    bool
}

func RoutePath(path string) (Route, bool) {
	switch {
	case path == "/_ping":
		return Route{Endpoint: EndpointPing, Capture: CaptureLocal, Local: true}, true
	case path == "/chat/completions":
		return Route{Endpoint: EndpointChat, Upstream: GitHubCopilotAPIHost, Capture: CaptureUsage}, true
	case path == "/agents" || strings.HasPrefix(path, "/agents/"):
		return Route{Endpoint: EndpointAgent, Upstream: GitHubCopilotAPIHost, Capture: CaptureUsage}, true
	case path == "/models":
		return Route{Endpoint: EndpointModels, Upstream: GitHubCopilotAPIHost, Capture: CaptureNone}, true
	case path == "/models/session":
		return Route{Endpoint: EndpointModelsSession, Upstream: GitHubCopilotAPIHost, Capture: CaptureNone}, true
	case path == "/responses":
		return Route{Endpoint: EndpointResponses, Upstream: GitHubCopilotAPIHost, Capture: CaptureTunnel}, true
	case path == "/embeddings":
		return Route{Endpoint: EndpointEmbeddings, Upstream: GitHubCopilotAPIHost, Capture: CaptureMetadata}, true
	case strings.HasPrefix(path, "/v1/engines/"):
		return Route{Endpoint: EndpointCompletions, Upstream: GitHubCopilotProxyHost, Capture: CaptureUsage}, true
	case path == "/v1/completions":
		return Route{Endpoint: EndpointCompletions, Upstream: GitHubCopilotProxyHost, Capture: CaptureUsage}, true
	case path == "/v1/messages" || strings.HasPrefix(path, "/v1/messages/"):
		return Route{Endpoint: EndpointChat, Upstream: GitHubCopilotAPIHost, Capture: CaptureUsage}, true
	default:
		return Route{}, false
	}
}
