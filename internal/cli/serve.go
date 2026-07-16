package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"copilot-monitoring/dashboard"
	"copilot-monitoring/internal/api"
	"copilot-monitoring/internal/store"
)

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7734", "HTTP listen address")
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	retentionDays := fs.Int("retention-days", 365, "days of requests and sessions to retain (0 disables)")
	anomalyRetentionDays := fs.Int("anomaly-retention-days", 30, "days of anomalies to retain (0 disables)")
	dryRun := fs.Bool("dry-run", false, "report retention deletions without executing them")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	stopRetention, retentionDryRun, err := startRetention(st, retentionConfig{
		requestDays: *retentionDays,
		anomalyDays: *anomalyRetentionDays,
		dryRun:      *dryRun,
	}, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "retention setup failed: %v\n", err)
		return 1
	}
	defer stopRetention()
	if retentionDryRun {
		return 0
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandler(st))
	mux.Handle("/", dashboard.Handler())

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
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
