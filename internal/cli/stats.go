package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"llm-proxy/internal/store"
)

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

func printStatsRows(w io.Writer, rows []store.ModelStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tENDPOINT\tREQUESTS\tPROMPT_TOK\tCACHED_TOK\tCACHE_WRITE_TOK\tCOMPL_TOK\tTOTAL\tTOKENS_REMOVED")
	for _, row := range rows {
		removed := "-"
		if row.CompressedRequests > 0 {
			removed = fmt.Sprintf("%d", row.CompressionRemovedTokens)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", row.Model, row.Endpoint, row.Requests, row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens, row.CompletionTokens, row.TotalTokens, removed)
	}
	_ = tw.Flush()
}
