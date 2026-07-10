package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"llm-proxy/internal/store"
)

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
