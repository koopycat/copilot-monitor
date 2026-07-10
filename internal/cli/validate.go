package cli

import (
	"flag"
	"fmt"
	"io"

	"llm-proxy/internal/proxy"
)

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	routesConfig := fs.String("routes-config", "", "JSON file with route definitions (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *routesConfig == "" {
		fmt.Fprintln(stderr, "error: --routes-config is required")
		return 2
	}

	cfg, err := proxy.LoadConfig(*routesConfig)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "routes config is valid (%d routes)\n", len(cfg.Routes))
	return 0
}
