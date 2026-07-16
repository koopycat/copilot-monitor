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
	case "rebuild-sessions":
		return runRebuildSessions(args[1:], stdout, stderr)
	case "live":
		return runLive(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
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
	fmt.Fprint(w, strings.TrimSpace(`copilot-monitor monitors LLM API usage through a local HTTP reverse proxy.

Usage:
  copilot-monitor run --upstream <host> [--addr 127.0.0.1:7733] [--headroom-proxy-addr 127.0.0.1:8787] [--db path] [--project name] [--usage-debug-log path] [--no-live] [--dashboard] [--retention-days 365] [--anomaly-retention-days 30] [--dry-run]
  copilot-monitor stats [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor cost [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor today [--db path] [--project name] [--endpoint chat]
  copilot-monitor sessions [--db path] [--since 30d] [--project name] [--limit 50]
  copilot-monitor rebuild-sessions [--db path] [--gap 30m] [--vacuum]
  copilot-monitor live [--db path] [--json] [--watch]
  copilot-monitor serve [--addr 127.0.0.1:7734] [--db path] [--retention-days 365] [--anomaly-retention-days 30] [--dry-run]
  copilot-monitor export [--since 30d] [--db path]
  copilot-monitor completion zsh
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener (also shows a live session tail when stderr is a TTY).
  serve             Start the read-only HTTP API and dashboard.
  stats             Print captured usage grouped by model and endpoint.
  cost              Print estimated equivalent provider list-price cost.
  today             Print today's captured usage.
  sessions          Print captured sessions.
  rebuild-sessions  Rebuild sessions from all requests (offline maintenance).
  live              Print the current active session (--watch to auto-refresh).
  export            Export captured request metadata to CSV.
  inspect           Show detected proxy anomalies (unrouted paths, parse errors, auth issues).
  completion        Generate shell completion scripts (currently zsh only).
  version           Print the version.
`)+"\n")
}
