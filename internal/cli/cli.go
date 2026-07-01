package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"copilot-monitoring/internal/proxy"
)

const version = "0.1.0-dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "version":
		fmt.Fprintf(stdout, "copilot-monitor %s\n", version)
		return 0
	case "configure-vscode":
		return runConfigure(args[1:], stdout, stderr)
	case "run":
		return runServer(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`copilot-monitor monitors GitHub Copilot model API usage through a local HTTP reverse proxy.

Usage:
  copilot-monitor run [--addr 127.0.0.1:7733]
  copilot-monitor configure-vscode [--addr 127.0.0.1:7733]
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener.
  configure-vscode  Print the VSCode settings JSON snippet.
  version           Print the version.
`)+"\n")
}

func runConfigure(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure-vscode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7733", "proxy listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	printVSCodeSettings(stdout, *addr)
	return 0
}

func printVSCodeSettings(w io.Writer, addr string) {
	baseURL := "http://" + settingsAddr(addr)
	fmt.Fprintf(w, "{\n")
	fmt.Fprintf(w, "  \"github.copilot.advanced\": {\n")
	fmt.Fprintf(w, "    \"debug.overrideProxyUrl\": %q,\n", baseURL)
	fmt.Fprintf(w, "    \"debug.overrideCapiUrl\": %q,\n", baseURL)
	fmt.Fprintf(w, "    \"authProvider\": \"github\"\n")
	fmt.Fprintf(w, "  }\n")
	fmt.Fprintf(w, "}\n")
}

func settingsAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func runServer(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7733", "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	handler := proxy.NewHandler(stderr)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "VSCode settings:\n")
	printVSCodeSettings(stdout, *addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}
