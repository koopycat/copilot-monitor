package proxy

// DefaultRoutes returns the built-in GitHub Copilot route configuration used
// when no --routes-config is provided.
func DefaultRoutes() *ProxyConfig {
	return &ProxyConfig{
		Routes: []RouteConfig{
			{
				Label:   "ping",
				Path:    "/_ping",
				Capture: "local",
			},
			{
				Provider:     "copilot",
				UpstreamHost: "api.githubcopilot.com",
				Capture:      "usage",
			},
			{
				Label:        "embeddings",
				Provider:     "copilot",
				Path:         "/embeddings",
				UpstreamHost: "api.githubcopilot.com",
				Capture:      "metadata",
			},
			{
				Label:        "completions",
				Provider:     "copilot",
				Path:         "/v1/engines",
				UpstreamHost: "copilot-proxy.githubusercontent.com",
				Capture:      "usage",
				PrefixMatch:  true,
				NotBilled:    true,
			},
			{
				Label:        "completions",
				Provider:     "copilot",
				Path:         "/v1/completions",
				UpstreamHost: "copilot-proxy.githubusercontent.com",
				Capture:      "usage",
				NotBilled:    true,
			},
		},
	}
}
