package cli

import (
	"fmt"
	"io"
	"strings"
)

var version = "0.1.0-dev"

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
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "cost":
		return runCost(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr)
	case "today":
		return runToday(args[1:], stdout, stderr)
	case "sessions":
		return runSessions(args[1:], stdout, stderr)
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

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`copilot-monitor monitors GitHub Copilot model API usage through a local HTTP reverse proxy.

Usage:
  copilot-monitor run [--addr 127.0.0.1:7733] [--db path] [--project name] [--usage-debug-log path]
  copilot-monitor configure-vscode [--addr 127.0.0.1:7733]
  copilot-monitor stats [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor cost [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor compare [--db path] [--a 2026-06] [--b 2026-07]
  copilot-monitor today [--db path] [--project name] [--endpoint chat]
  copilot-monitor sessions [--db path] [--since 30d] [--project name] [--limit 50]
  copilot-monitor serve [--addr 127.0.0.1:7734] [--db path]
  copilot-monitor export [--since 30d] [--db path]
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener.
  configure-vscode  Print the VSCode settings JSON snippet.
  serve             Start the read-only HTTP API and dashboard.
  stats             Print captured usage grouped by model and endpoint.
  cost              Print estimated equivalent provider list-price cost.
  compare           Compare estimated cost and tokens for two months.
  today             Print today's captured usage.
  sessions          Print captured sessions using a 30-minute inactivity gap.
  export            Export captured request metadata to CSV.
  version           Print the version.
`)+"\n")
}
