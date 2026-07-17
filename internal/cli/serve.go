package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	logFormat := fs.String("log-format", "human", "accepted for compatibility, not yet used by serve")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// log-format is accepted for CLI compatibility with run but has no effect in serve.
	_ = logFormat

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening database %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	stopRetention, retentionDryRun, err := startRetention(st, retentionConfig{
		requestDays: *retentionDays,
		anomalyDays: *anomalyRetentionDays,
		dryRun:      *dryRun,
	}, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: retention setup: %v\n", err)
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sigReceived := make(chan os.Signal, 1)
	go func() {
		s := <-sigCh
		sigReceived <- s
		fmt.Fprintf(stderr, "shutting down...\n")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "error: server: %v\n", err)
		return 1
	}

	signal.Stop(sigCh)
	select {
	case s := <-sigReceived:
		if s == syscall.SIGINT {
			return 130
		}
	default:
	}
	return 0
}
