package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"copilot-monitoring/internal/store"
)

func runToday(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("today", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	project := fs.String("project", projectDefault(), "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening database %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	rows, err := st.Stats(context.Background(), store.StatsFilter{Since: start, Project: *project, Endpoint: *endpoint})
	if err != nil {
		fmt.Fprintf(stderr, "error: querying today's stats: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			fmt.Fprintf(stderr, "error: encoding json: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Usage for %s\n", start.Format("January 2, 2006"))
	printStatsRows(stdout, rows)
	return 0
}
