package proxy

import (
	"path"
	"sort"
	"strings"
)

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
	Endpoint           Endpoint
	Upstream           string
	UpstreamPathPrefix string
	Capture            CaptureMode
	Local              bool
	Models             []string
}

// matchModelPattern returns true if the given model name matches the pattern.
// Patterns ending with "*" perform prefix matching (e.g., "gpt-*" matches "gpt-4o").
// Patterns containing "*" elsewhere perform glob matching via path.Match.
// Otherwise an exact string match is required.
func matchModelPattern(pattern, model string) bool {
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	if strings.Contains(pattern, "*") {
		// path.Match supports * wildcards anywhere in the pattern.
		ok, _ := path.Match(pattern, model)
		return ok
	}
	return pattern == model
}

// routeMatchesModel returns true if the route should handle this model.
// A route with nil/empty Models matches any model.
func (r Route) routeMatchesModel(model string) bool {
	if len(r.Models) == 0 {
		return true
	}
	if model == "" {
		return false
	}
	for _, pattern := range r.Models {
		if matchModelPattern(pattern, model) {
			return true
		}
	}
	return false
}

func copilotRoutePath(path string) (Route, bool) {
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
		return Route{Endpoint: EndpointResponses, Upstream: GitHubCopilotAPIHost, Capture: CaptureUsage}, true
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

type prefixRoute struct {
	prefix string
	route  Route
}

// modelExactRoute pairs an exact path with a Route, for ordered model-based matching.
type modelExactRoute struct {
	path  string
	route Route
}

// modelPrefixRoute pairs a prefix with a Route, for ordered model-based matching.
type modelPrefixRoute struct {
	prefix string
	route  Route
}

type Router struct {
	configRoutes      map[string]Route
	prefixRoutes      []prefixRoute
	modelRoutes       []modelExactRoute
	modelPrefixRoutes []modelPrefixRoute
}

func NewRouter(cfg *ProxyConfig) *Router {
	r := &Router{
		configRoutes: make(map[string]Route),
	}
	if cfg == nil {
		return r
	}
	for _, rc := range cfg.Routes {
		endpoint := Endpoint(rc.Path)
		if rc.Label != "" {
			endpoint = Endpoint(rc.Label)
		}
		route := Route{
			Endpoint:           endpoint,
			Upstream:           rc.UpstreamHost,
			UpstreamPathPrefix: rc.UpstreamPathPrefix,
			Capture:            CaptureMode(rc.Capture),
			Models:             rc.Models,
		}
		if rc.PrefixMatch {
			r.prefixRoutes = append(r.prefixRoutes, prefixRoute{
				prefix: rc.Path,
				route:  route,
			})
			r.modelPrefixRoutes = append(r.modelPrefixRoutes, modelPrefixRoute{
				prefix: rc.Path,
				route:  route,
			})
		} else {
			r.configRoutes[rc.Path] = route
			r.modelRoutes = append(r.modelRoutes, modelExactRoute{
				path:  rc.Path,
				route: route,
			})
		}
	}
	sort.Slice(r.prefixRoutes, func(i, j int) bool {
		return len(r.prefixRoutes[i].prefix) > len(r.prefixRoutes[j].prefix)
	})
	sort.Slice(r.modelPrefixRoutes, func(i, j int) bool {
		return len(r.modelPrefixRoutes[i].prefix) > len(r.modelPrefixRoutes[j].prefix)
	})
	return r
}

// Match returns the Route for the given path, ignoring model fields.
// This is used by combinedDashProxy in run.go for path-only matching.
func (r *Router) Match(path string) (Route, bool) {
	route, ok := r.configRoutes[path]
	if ok {
		return route, true
	}
	for _, pr := range r.prefixRoutes {
		if strings.HasPrefix(path, pr.prefix) {
			return pr.route, true
		}
	}
	return copilotRoutePath(path)
}

// MatchModel returns the Route for the given path and model.
// Model-based filtering is applied on top of path matching.
// Routes are checked in insertion order; the first matching route wins.
// Routes without Models match any model, so a catch-all route after model-specific
// routes acts as the default fallback.
func (r *Router) MatchModel(path, model string) (Route, bool) {
	// Exact path match with model filter (insertion order)
	for _, mr := range r.modelRoutes {
		if mr.path == path && mr.route.routeMatchesModel(model) {
			return mr.route, true
		}
	}
	// Prefix match with model filter (longest prefix first)
	for _, pr := range r.modelPrefixRoutes {
		if strings.HasPrefix(path, pr.prefix) && pr.route.routeMatchesModel(model) {
			return pr.route, true
		}
	}
	// Built-in Copilot fallback (no model filtering)
	return copilotRoutePath(path)
}

func (r Route) ApplyPathPrefix(inPath, inRawPath string) (path, rawPath string) {
	if r.UpstreamPathPrefix != "" {
		path = r.UpstreamPathPrefix + inPath
		if inRawPath != "" {
			rawPath = r.UpstreamPathPrefix + inRawPath
		}
		return
	}
	return inPath, inRawPath
}
