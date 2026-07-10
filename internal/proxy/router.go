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

// KnownProviders lists the recognized URL path prefixes for provider routing.
var KnownProviders = map[string]bool{
	"copilot": true,
	"openai":  true,
	"kilo":    true,
}

// StripProviderPrefix extracts a known provider prefix from the first path segment
// and returns the provider name with the remaining path.
// "/copilot/chat/completions" → ("copilot", "/chat/completions")
// "/copilot" → ("copilot", "/")
// "/unknown/path" → ("", "/unknown/path")
func StripProviderPrefix(urlPath string) (provider, remaining string) {
	if urlPath == "" || urlPath == "/" {
		return "", urlPath
	}
	rest := strings.TrimPrefix(urlPath, "/")
	idx := strings.Index(rest, "/")
	var first string
	if idx < 0 {
		first = rest
		rest = ""
	} else {
		first = rest[:idx]
		rest = rest[idx:]
	}
	if KnownProviders[first] {
		if rest == "" {
			return first, "/"
		}
		return first, rest
	}
	return "", urlPath
}

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
	Provider           string
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
func (r Route) routeMatchesProvider(detectedProvider string) bool {
	if r.Provider == "" {
		return true
	}
	return r.Provider == detectedProvider
}

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

// routeEntry holds a single route entry which is either an exact path match or a prefix match.
// Exactly one of path or prefix is non-empty.
type routeEntry struct {
	path   string // exact path if non-empty
	prefix string // prefix match if non-empty
	route  Route
}

func (e routeEntry) routeMatchesProvider(provider string) bool {
	return e.route.Provider == "" || e.route.Provider == provider
}

type Router struct {
	exactRoutes map[string]Route // fast lookup for path-only exact matches
	entries     []routeEntry     // exact entries (insertion order) then prefix entries (longest first)
}

func NewRouter(cfg *ProxyConfig) *Router {
	r := &Router{
		exactRoutes: make(map[string]Route),
	}
	if cfg == nil {
		return r
	}
	var exactEntries []routeEntry
	var prefixEntries []routeEntry
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
			Provider:           rc.Provider,
		}
		if rc.PrefixMatch {
			prefixEntries = append(prefixEntries, routeEntry{
				prefix: rc.Path,
				route:  route,
			})
		} else {
			r.exactRoutes[rc.Path] = route
			exactEntries = append(exactEntries, routeEntry{
				path:  rc.Path,
				route: route,
			})
		}
	}
	// Sort prefix entries by longest prefix first
	sort.Slice(prefixEntries, func(i, j int) bool {
		return len(prefixEntries[i].prefix) > len(prefixEntries[j].prefix)
	})
	r.entries = append(exactEntries, prefixEntries...)
	return r
}

// Match returns the Route for the given path, ignoring model fields.
// This is used by combinedDashProxy in run.go for path-only matching.
func (r *Router) Match(urlPath string) (Route, bool) {
	_, urlPath = StripProviderPrefix(urlPath)
	if route, ok := r.exactRoutes[urlPath]; ok {
		return route, true
	}
	for _, e := range r.entries {
		if e.prefix != "" && strings.HasPrefix(urlPath, e.prefix) {
			return e.route, true
		}
	}
	return copilotRoutePath(urlPath)
}

// MatchModel returns the Route for the given path and model.
// Model-based filtering is applied on top of path matching.
// Routes are checked in insertion order; the first matching route wins.
// Routes without Models match any model, so a catch-all route after model-specific
// routes acts as the default fallback.
func (r *Router) MatchModel(path, model, provider string) (Route, bool) {
	// Exact path match with model and provider filter (insertion order)
	for _, e := range r.entries {
		if e.path != "" && e.path == path && e.routeMatchesProvider(provider) && e.route.routeMatchesModel(model) {
			return e.route, true
		}
	}
	// Prefix match with model and provider filter (longest prefix first)
	for _, e := range r.entries {
		if e.prefix != "" && strings.HasPrefix(path, e.prefix) && e.routeMatchesProvider(provider) && e.route.routeMatchesModel(model) {
			return e.route, true
		}
	}
	// Built-in Copilot fallback — only applies when no explicit provider or copilot
	if provider == "" || provider == "copilot" {
		return copilotRoutePath(path)
	}
	return Route{}, false
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
