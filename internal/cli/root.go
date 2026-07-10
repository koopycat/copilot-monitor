package cli

import (
	"fmt"
	"io"
	"strings"
)

var version = "0.1.0-dev"

const binaryName = "copilot-monitor"

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
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
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
	fmt.Fprint(w, strings.TrimSpace(`copilot-monitor monitors GitHub Copilot model API usage through a local HTTP reverse proxy.

Usage:
  copilot-monitor run [--addr 127.0.0.1:7733] [--db path] [--project name] [--usage-debug-log path] [--no-live] [--dashboard]
  copilot-monitor stats [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor cost [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor today [--db path] [--project name] [--endpoint chat]
  copilot-monitor sessions [--db path] [--since 30d] [--project name] [--limit 50]
  copilot-monitor live [--db path] [--json] [--watch]
  copilot-monitor serve [--addr 127.0.0.1:7734] [--db path]
  copilot-monitor export [--since 30d] [--db path]
  copilot-monitor init [--force]
  copilot-monitor validate --routes-config path.json
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener (also shows a live session tail when stderr is a TTY).
  serve             Start the read-only HTTP API and dashboard.
  stats             Print captured usage grouped by model and endpoint.
  cost              Print estimated equivalent provider list-price cost.
  today             Print today's captured usage.
  sessions          Print captured sessions using a 30-minute inactivity gap.
  live              Print the current active session (--watch to auto-refresh).
  export            Export captured request metadata to CSV.
  init              Create a starter routes.json config file.
  validate          Validate a routes config file.
  version           Print the version.
`)+"\n")
}
