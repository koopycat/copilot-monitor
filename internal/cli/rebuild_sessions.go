package cli

import (
	"context"
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
		return 2
	}
	if *gap <= 0 {
		fmt.Fprintln(stderr, "--gap must be positive")
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	ctx := context.Background()
	if err := st.RebuildSessions(ctx, *gap); err != nil {
		fmt.Fprintf(stderr, "failed to rebuild sessions: %v\n", err)
		return 1
	}
	if *vacuum {
		if err := st.Vacuum(ctx); err != nil {
			fmt.Fprintf(stderr, "failed to vacuum database: %v\n", err)
			return 1
		}
	}
	fmt.Fprintln(stdout, "sessions rebuilt")
	return 0
}
