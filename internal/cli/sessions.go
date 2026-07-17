package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"copilot-monitoring/internal/store"
)

func runSessions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back, e.g. 24h, 7d, 30d, or all")
	project := fs.String("project", projectDefault(), "filter by project")
	limit := fs.Int("limit", 50, "maximum sessions to print")
	cursor := fs.String("cursor", "", "cursor started_at for pagination (format: RFC3339)")
	cursorID := fs.Int64("cursor-id", 0, "cursor session ID for pagination")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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

	var cursorStartedAt time.Time
	if *cursor != "" {
		cursorStartedAt, err = time.Parse(time.RFC3339, *cursor)
		if err != nil {
			fmt.Fprintf(stderr, "invalid --cursor value %q: %v\n", *cursor, err)
			return 2
		}
	}
	rows, err := st.Sessions(context.Background(), store.SessionFilter{
		Since:           since,
		Project:         *project,
		Limit:           *limit,
		CursorStartedAt: cursorStartedAt,
		CursorID:        *cursorID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: querying sessions: %v\n", err)
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

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "START\tEND\tDURATION\tPROJECT\tREQUESTS\tTOKENS\tTOKENS_REMOVED")
	for _, row := range rows {
		duration := row.EndedAt.Sub(row.StartedAt).Round(time.Second)
		removed := "-"
		if models, err := st.SessionModels(context.Background(), row.ID); err == nil {
			var total int
			for _, m := range models {
				total += m.CompressionRemovedTokens
			}
			if total > 0 {
				removed = fmt.Sprintf("%d", total)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
			row.StartedAt.Local().Format("2006-01-02 15:04:05"),
			row.EndedAt.Local().Format("2006-01-02 15:04:05"),
			duration,
			emptyDash(row.Project),
			row.RequestCount,
			row.TokenCount,
			removed,
		)
	}
	_ = tw.Flush()
	return 0
}
