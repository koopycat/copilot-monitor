package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite existing routes config")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	configDir := xdgConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(stderr, "error: failed to create config directory %q: %v\n", configDir, err)
		return 1
	}

	configPath := filepath.Join(configDir, "routes.json")

	// Check if file exists (unless --force).
	if _, err := os.Stat(configPath); err == nil && !*force {
		fmt.Fprintf(stderr, "error: %q already exists (use --force to overwrite)\n", configPath)
		return 1
	}

	routes := buildInitRoutes()

	data, err := json.MarshalIndent(struct {
		Routes []routeStub `json:"routes"`
	}{Routes: routes}, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: failed to generate config: %v\n", err)
		return 1
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		fmt.Fprintf(stderr, "error: failed to write %q: %v\n", configPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "Created %s\n", configPath)
	fmt.Fprintf(stdout, "Run: copilot-monitor run --routes-config %s\n", configPath)
	return 0
}

type routeStub struct {
	Label        string   `json:"label,omitempty"`
	Path         string   `json:"path"`
	UpstreamHost string   `json:"upstream_host"`
	Capture      string   `json:"capture"`
	Provider     string   `json:"provider,omitempty"`
	PrefixMatch  bool     `json:"prefix_match,omitempty"`
	Models       []string `json:"models,omitempty"`
}

func buildInitRoutes() []routeStub {
	routes := []routeStub{
		{
			Label:        "ping",
			Path:         "/_ping",
			UpstreamHost: "",
			Capture:      "local",
		},
	}

	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
	hasAnthropic := os.Getenv("ANTHROPIC_API_KEY") != ""

	if hasOpenAI {
		routes = append(routes, routeStub{
			Label:        "chat",
			Path:         "/v1/chat/completions",
			UpstreamHost: "api.openai.com",
			Capture:      "usage",
			Provider:     "openai",
		})
	}

	if hasAnthropic {
		routes = append(routes, routeStub{
			Label:        "chat",
			Path:         "/v1/messages",
			UpstreamHost: "api.anthropic.com",
			Capture:      "usage",
			Provider:     "anthropic",
		})
	}

	if !hasOpenAI && !hasAnthropic {
		// No API keys found — include a generic OpenAI-compatible stub.
		routes = append(routes, routeStub{
			Label:        "chat",
			Path:         "/v1/chat/completions",
			UpstreamHost: "api.openai.com",
			Capture:      "usage",
			Provider:     "openai",
			Models:       []string{"gpt-4o", "gpt-4o-mini"},
		})
	}

	return routes
}

// xdgConfigDir returns $XDG_CONFIG_HOME/copilot-monitor, falling back to ~/.config/copilot-monitor.
func xdgConfigDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "copilot-monitor")
}
