package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"copilot-monitoring/internal/api"
	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/log"
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
	case "cost":
		return runCost(args[1:], stdout, stderr)
	case "today":
		return runToday(args[1:], stdout, stderr)
	case "sessions":
		return runSessions(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
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
  copilot-monitor today [--db path] [--project name] [--endpoint chat]
  copilot-monitor sessions [--db path] [--since 30d] [--project name] [--limit 50]
  copilot-monitor serve [--addr 127.0.0.1:7734] [--db path]
  copilot-monitor version

Commands:
  run               Start the local HTTP proxy listener.
  configure-vscode  Print the VSCode settings JSON snippet.
  serve             Start the read-only HTTP API and dashboard.
  stats             Print captured usage grouped by model and endpoint.
  cost              Print estimated equivalent provider list-price cost.
  today             Print today's captured usage.
  sessions          Print captured sessions using a 30-minute inactivity gap.
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
	usageDebugPath := fs.String("usage-debug-log", "", "optional JSONL path for usage-only debug metadata")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	usageDebug, err := proxy.OpenUsageDebugLogger(*usageDebugPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open usage debug log %q: %v\n", *usageDebugPath, err)
		return 1
	}
	defer usageDebug.Close()

	logWriter := log.NewWriter(stderr)
	handler := proxy.NewHandlerWithStoreAndUsageDebug(logWriter, st, *project, usageDebug)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))
	if *usageDebugPath != "" {
		fmt.Fprintf(stdout, "usage debug log: %s\n", store.FormatPath(*usageDebugPath))
	}
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
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
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

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}
	printStatsRows(stdout, rows)
	return 0
}

func runCost(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back, e.g. 24h, 7d, 30d, or all")
	project := fs.String("project", "", "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
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
	cat, err := catalog.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load model catalog: %v\n", err)
		return 1
	}
	total := costcalc.Calculate(rows, cat)

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(total); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Estimated equivalent GitHub Copilot AI-credit list-price cost (%s). This is not your GitHub Copilot bill.\n", cat.Currency)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tENDPOINT\tPROVIDER\tREQUESTS\tINPUT_TOK\tCACHED_TOK\tCACHE_WRITE_TOK\tOUTPUT_TOK\tINPUT $\tCACHED $\tCACHE WRITE $\tOUTPUT $\tEST. LIST $")
	for _, row := range total.Rows {
		provider := row.Provider
		if row.Fallback {
			provider += "*"
		}
		if row.NotBilled {
			provider += " (not billed)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%.6f\t%.6f\t%.6f\t%.6f\t%.6f\n", row.Model, row.Endpoint, provider, row.Requests, row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens, row.CompletionTokens, row.InputUSD, row.CachedInputUSD, row.CacheWriteUSD, row.OutputUSD, row.TotalUSD)
	}
	fmt.Fprintf(tw, "TOTAL\t\t\t%d\t%d\t%d\t%d\t%d\t%.6f\t%.6f\t%.6f\t%.6f\t%.6f\n", total.Requests, total.PromptTokens, total.CachedInputTokens, total.CacheWriteTokens, total.CompletionTokens, total.InputUSD, total.CachedInputUSD, total.CacheWriteUSD, total.OutputUSD, total.TotalUSD)
	_ = tw.Flush()
	if total.FallbackCount > 0 {
		fmt.Fprintf(stdout, "\n* provider or generic fallback pricing used for %d row(s).\n", total.FallbackCount)
	}
	if total.NotBilledCount > 0 {
		fmt.Fprintf(stdout, "Inline code completion rows are shown with zero AI-credit cost because GitHub docs say code completions are not billed in AI credits.\n")
	}
	return 0
}

func runToday(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("today", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	project := fs.String("project", "", "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	rows, err := st.Stats(context.Background(), store.StatsFilter{Since: start, Project: *project, Endpoint: *endpoint})
	if err != nil {
		fmt.Fprintf(stderr, "failed to query today's stats: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Usage since %s\n", start.Format(time.RFC3339))
	printStatsRows(stdout, rows)
	return 0
}

func runSessions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back, e.g. 24h, 7d, 30d, or all")
	project := fs.String("project", "", "filter by project")
	limit := fs.Int("limit", 50, "maximum sessions to print")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
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

	if err := st.RebuildSessions(context.Background(), 30*time.Minute); err != nil {
		fmt.Fprintf(stderr, "failed to rebuild sessions: %v\n", err)
		return 1
	}
	rows, err := st.Sessions(context.Background(), store.SessionFilter{Since: since, Project: *project, Limit: *limit})
	if err != nil {
		fmt.Fprintf(stderr, "failed to query sessions: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "START\tEND\tDURATION\tPROJECT\tREQUESTS\tTOKENS")
	for _, row := range rows {
		duration := row.EndedAt.Sub(row.StartedAt).Round(time.Second)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\n",
			row.StartedAt.Local().Format("2006-01-02 15:04:05"),
			row.EndedAt.Local().Format("2006-01-02 15:04:05"),
			duration,
			emptyDash(row.Project),
			row.RequestCount,
			row.TokenCount,
		)
	}
	_ = tw.Flush()
	return 0
}

func printStatsRows(w io.Writer, rows []store.ModelStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tENDPOINT\tREQUESTS\tPROMPT_TOK\tCACHED_TOK\tCACHE_WRITE_TOK\tCOMPL_TOK\tTOTAL")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n", row.Model, row.Endpoint, row.Requests, row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens, row.CompletionTokens, row.TotalTokens)
	}
	_ = tw.Flush()
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7734", "HTTP listen address")
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	handler := api.NewHandler(st)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor API listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
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
