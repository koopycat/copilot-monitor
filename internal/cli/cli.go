package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"
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
	case "stats":
		return runStats(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`copilot-monitor monitors GitHub Copilot model API usage through a local HTTP reverse proxy.

Usage:
  copilot-monitor run [--addr 127.0.0.1:7733] [--db path] [--project name]
  copilot-monitor configure-vscode [--addr 127.0.0.1:7733]
  copilot-monitor stats [--db path] [--since 30d] [--project name] [--endpoint chat]
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener.
  configure-vscode  Print the VSCode settings JSON snippet.
  stats             Print captured usage grouped by model and endpoint.
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
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	project := fs.String("project", "", "optional project label")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	handler := proxy.NewHandlerWithStore(stderr, st, *project)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))
	fmt.Fprintf(stdout, "VSCode settings:\n")
	printVSCodeSettings(stdout, *addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}

func runStats(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back, e.g. 24h, 7d, 30d, or all")
	project := fs.String("project", "", "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	since, err := parseSince(*sinceText, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "invalid --since value %q: %v\n", *sinceText, err)
		return 2
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	rows, err := st.Stats(context.Background(), store.StatsFilter{Since: since, Project: *project, Endpoint: *endpoint})
	if err != nil {
		fmt.Fprintf(stderr, "failed to query stats: %v\n", err)
		return 1
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tENDPOINT\tREQUESTS\tPROMPT_TOK\tCOMPL_TOK\tTOTAL")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\n", row.Model, row.Endpoint, row.Requests, row.PromptTokens, row.CompletionTokens, row.TotalTokens)
	}
	_ = tw.Flush()
	return 0
}

func parseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "all" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(value, "d") {
		daysText := strings.TrimSuffix(value, "d")
		var days int
		if _, err := fmt.Sscanf(daysText, "%d", &days); err != nil {
			return time.Time{}, err
		}
		if days < 0 {
			return time.Time{}, fmt.Errorf("duration must be non-negative")
		}
		return now.Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return time.Time{}, err
	}
	if d < 0 {
		return time.Time{}, fmt.Errorf("duration must be non-negative")
	}
	return now.Add(-d), nil
}
