package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"copilot-monitoring/internal/store"
)

func runRebuildSessions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rebuild-sessions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	gap := fs.Duration("gap", 30*time.Minute, "inactivity gap used to split sessions")
	vacuum := fs.Bool("vacuum", false, "compact the database after rebuilding sessions")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *gap <= 0 {
		fmt.Fprintln(stderr, "error: --gap must be positive")
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening database %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	ctx := context.Background()
	requestCount, err := st.RequestCount(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: counting requests: %v\n", err)
		return 1
	}

	start := time.Now()
	if err := st.RebuildSessions(ctx, *gap); err != nil {
		fmt.Fprintf(stderr, "error: rebuilding sessions: %v\n", err)
		return 1
	}
	elapsed := time.Since(start).Round(time.Millisecond)

	sessionCount, err := st.CountSessions(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "error: counting sessions: %v\n", err)
		return 1
	}
	if *vacuum {
		if err := st.Vacuum(ctx); err != nil {
			fmt.Fprintf(stderr, "error: vacuuming database: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "rebuilt %d sessions from %d requests in %s\n", sessionCount, requestCount, elapsed)
	return 0
}
