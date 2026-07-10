package cli

import (
	"fmt"
	"io"
	"strings"
)

var version = "0.1.0-dev"

const binaryName = "llm-proxy"

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
		fmt.Fprintf(stdout, "%s %s\n", binaryName, version)
		return 0
	case "run":
		return runServer(args[1:], stdout, stderr)
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "cost":
		return runCost(args[1:], stdout, stderr)
	case "today":
		return runToday(args[1:], stdout, stderr)
	case "sessions":
		return runSessions(args[1:], stdout, stderr)
	case "live":
		return runLive(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func settingsAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`llm-proxy monitors LLM API usage through a local HTTP reverse proxy.

Usage:
  llm-proxy run [--addr 127.0.0.1:7733] [--db path] [--project name] [--usage-debug-log path] [--no-live] [--dashboard]
  llm-proxy stats [--db path] [--since 30d] [--project name] [--endpoint chat]
  llm-proxy cost [--db path] [--since 30d] [--project name] [--endpoint chat]
  llm-proxy today [--db path] [--project name] [--endpoint chat]
  llm-proxy sessions [--db path] [--since 30d] [--project name] [--limit 50]
  llm-proxy live [--db path] [--json] [--watch]
  llm-proxy serve [--addr 127.0.0.1:7734] [--db path]
  llm-proxy export [--since 30d] [--db path]
  llm-proxy version

Commands:
  run               Start the local HTTP proxy listener (also shows a live session tail when stderr is a TTY).
  serve             Start the read-only HTTP API and dashboard.
  stats             Print captured usage grouped by model and endpoint.
  cost              Print estimated equivalent provider list-price cost.
  today             Print today's captured usage.
  sessions          Print captured sessions using a 30-minute inactivity gap.
  live              Print the current active session (--watch to auto-refresh).
  export            Export captured request metadata to CSV.
  version           Print the version.
`)+"\n")
}
