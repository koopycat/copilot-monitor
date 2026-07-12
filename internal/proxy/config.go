package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RouteCompression struct {
	Endpoint             string   `json:"endpoint"`
	Required             bool     `json:"required,omitempty"`
	CompressUserMessages bool     `json:"compress_user_messages,omitempty"`
	TargetRatio          *float64 `json:"target_ratio,omitempty"`
}

type RouteConfig struct {
	Provider           string            `json:"provider,omitempty"`
	Label              string            `json:"label,omitempty"`
	Path               string            `json:"path,omitempty"`
	UpstreamHost       string            `json:"upstream_host,omitempty"`
	UpstreamPathPrefix string            `json:"upstream_path_prefix,omitempty"`
	Capture            string            `json:"capture,omitempty"`
	PrefixMatch        bool              `json:"prefix_match,omitempty"`
	Models             []string          `json:"models,omitempty"`
	NotBilled          bool              `json:"not_billed,omitempty"`
	Compression        *RouteCompression `json:"compression,omitempty"`
}

// isProviderDefault returns true when the route has no path but has
// a provider set (attempted provider default, regardless of upstream_host).
func (rc *RouteConfig) isProviderDefault() bool {
	return rc.Path == "" && rc.Provider != ""
}

type ProxyConfig struct {
	Routes []RouteConfig `json:"routes"`
}

func LoadConfig(path string) (*ProxyConfig, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routes config %q: %w", path, err)
	}
	data = stripJSONComments(data)
	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse routes config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate routes config %q: %w", path, err)
	}
	return &cfg, nil
}

func routeID(i int, rc *RouteConfig) string {
	if rc.Label != "" {
		return fmt.Sprintf("route %q (%s)", rc.Label, rc.Path)
	}
	return fmt.Sprintf("route %q", rc.Path)
}

func (c *ProxyConfig) Validate() error {
	seenDefaults := make(map[string]int) // provider -> first index
	for i := range c.Routes {
		rc := &c.Routes[i]
		rc.Path = strings.TrimSpace(rc.Path)
		rc.UpstreamHost = strings.TrimSpace(rc.UpstreamHost)

		id := routeID(i, rc)

		// Provider default routes: no path, must have provider + upstream_host.
		if rc.isProviderDefault() {
			if rc.UpstreamHost == "" {
				return fmt.Errorf("route at index %d: provider default route requires upstream_host", i)
			}
			if firstIdx, exists := seenDefaults[rc.Provider]; exists {
				return fmt.Errorf("route at index %d: duplicate provider default for %q (first defined at index %d)", i, rc.Provider, firstIdx)
			}
			seenDefaults[rc.Provider] = i
			continue
		}

		if rc.Path == "" {
			return fmt.Errorf("%s: path is required", id)
		}
		if !strings.HasPrefix(rc.Path, "/") {
			return fmt.Errorf("%s: path must start with /", id)
		}
		if rc.UpstreamHost == "" && rc.Capture != "local" {
			return fmt.Errorf("%s: upstream_host is required (unless capture is 'local')", id)
		}
		if rc.Label != "" && strings.TrimSpace(rc.Label) == "" {
			return fmt.Errorf("%s: label must not be empty", id)
		}
		rc.UpstreamPathPrefix = strings.TrimRight(rc.UpstreamPathPrefix, "/")

		// Validate model patterns
		for j, m := range rc.Models {
			m = strings.TrimSpace(m)
			if m == "" {
				return fmt.Errorf("%s: models[%d] must not be empty", id, j)
			}
			rc.Models[j] = m
		}

		if rc.Provider != "" && strings.TrimSpace(rc.Provider) == "" {
			return fmt.Errorf("%s: provider must not be empty if set", id)
		}

		switch rc.Capture {
		case "usage", "metadata", "none", "tunnel", "local":
			// valid
		default:
			return fmt.Errorf("%s: unknown capture mode %q", id, rc.Capture)
		}
	}
	return nil
}

// stripJSONComments removes // line comments and /* block comments */ from JSON data.
// It handles strings by tracking whether the parser is inside a string literal,
// so that comment delimiters inside strings are not treated as comments.
// Block comments are properly nested and tracked.
func stripJSONComments(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	inString := false
	escape := false
	inBlockComment := false

	for i < len(data) {
		b := data[i]

		if inBlockComment {
			if b == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i += 2
			} else {
				i++
			}
			continue
		}

		if inString {
			result = append(result, b)
			if escape {
				escape = false
			} else if b == '\\' {
				escape = true
			} else if b == '"' {
				inString = false
			}
			i++
			continue
		}

		// Not in string, check for comment starts
		if b == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				// Line comment: skip until newline
				i += 2
				for i < len(data) && data[i] != '\n' {
					i++
				}
				continue
			}
			if data[i+1] == '*' {
				// Block comment
				inBlockComment = true
				i += 2
				continue
			}
		}

		if b == '"' {
			inString = true
		}

		result = append(result, b)
		i++
	}

	return result
}
