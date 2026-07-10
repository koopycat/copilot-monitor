package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RouteConfig struct {
	Provider           string   `json:"provider,omitempty"`
	Label              string   `json:"label,omitempty"`
	Path               string   `json:"path"`
	UpstreamHost       string   `json:"upstream_host"`
	UpstreamPathPrefix string   `json:"upstream_path_prefix,omitempty"`
	Capture            string   `json:"capture"`
	PrefixMatch        bool     `json:"prefix_match,omitempty"`
	Models             []string `json:"models,omitempty"`
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
	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse routes config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate routes config %q: %w", path, err)
	}
	return &cfg, nil
}

func (c *ProxyConfig) Validate() error {
	for i := range c.Routes {
		rc := &c.Routes[i]
		rc.Path = strings.TrimSpace(rc.Path)
		rc.UpstreamHost = strings.TrimSpace(rc.UpstreamHost)

		if rc.Path == "" {
			return fmt.Errorf("route %d: path is required", i)
		}
		if !strings.HasPrefix(rc.Path, "/") {
			return fmt.Errorf("route %d (%q): path must start with /", i, rc.Path)
		}
		if rc.UpstreamHost == "" {
			return fmt.Errorf("route %d (%q): upstream_host is required", i, rc.Path)
		}
		if rc.Label != "" && strings.TrimSpace(rc.Label) == "" {
			return fmt.Errorf("route %d (%q): label must not be empty", i, rc.Path)
		}
		rc.UpstreamPathPrefix = strings.TrimRight(rc.UpstreamPathPrefix, "/")

		// Validate model patterns
		for j, m := range rc.Models {
			m = strings.TrimSpace(m)
			if m == "" {
				return fmt.Errorf("route %d (%q): models[%d] must not be empty", i, rc.Path, j)
			}
			rc.Models[j] = m
		}

		switch rc.Capture {
		case "usage", "metadata", "none", "tunnel", "local":
			// valid
		default:
			return fmt.Errorf("route %d (%q): unknown capture mode %q", i, rc.Path, rc.Capture)
		}
	}
	return nil
}
