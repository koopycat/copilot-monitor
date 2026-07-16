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
	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"
)

func runServer(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7733", "HTTP listen address")
	upstream := fs.String("upstream", "", "upstream host to proxy requests to (required, e.g. api.githubcopilot.com)")
	headroomProxyAddr := fs.String("headroom-proxy-addr", "127.0.0.1:8787", "headroom compression proxy address")
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	project := fs.String("project", "", "optional project label")
	usageDebugPath := fs.String("usage-debug-log", "", "optional JSONL path for usage-only debug metadata")
	rawLogPath := fs.String("raw-log", "", "optional JSONL path for raw request debugging (logs truncated bodies, headers; treat output as sensitive)")
	noLive := fs.Bool("no-live", false, "disable the live session tail below the startup banner")
	dashboardFlag := fs.Bool("dashboard", false, "start dashboard API and UI on a separate port (7734)")
	retentionDays := fs.Int("retention-days", 365, "days of requests and sessions to retain (0 disables)")
	anomalyRetentionDays := fs.Int("anomaly-retention-days", 30, "days of anomalies to retain (0 disables)")
	dryRun := fs.Bool("dry-run", false, "report retention deletions without executing them")
	logFormat := fs.String("log-format", "human", "log output format: human (rich colored output, default) or json (one JSON object per line)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *upstream == "" {
		fmt.Fprintln(stderr, "error: --upstream is required")
		return 1
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

	usageDebug, err := proxy.OpenUsageDebugLogger(*usageDebugPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open usage debug log %q: %v\n", *usageDebugPath, err)
		return 1
	}
	defer usageDebug.Close()

	rawLogger, err := proxy.OpenRawLogger(*rawLogPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open raw debug log %q: %v\n", *rawLogPath, err)
		return 1
	}
	defer rawLogger.Close()

	if *rawLogPath != "" {
		fmt.Fprintf(stderr, "raw debug logging is enabled: request bodies (up to %d bytes) are written to %s. This file may contain source code and prompts. Treat it as sensitive.\n", 1024, *rawLogPath)
	}

	// First-line startup banner — must appear before any other output.
	fmt.Fprintf(stderr, "copilot-monitor: listening on %s upstream=%s headroom=%s - curl http://%s/_ping\n", settingsAddr(*addr), *upstream, *headroomProxyAddr, settingsAddr(*addr))

	// Validate log format.
	var lf log.LogFormat
	switch *logFormat {
	case "human", "json":
		lf = log.LogFormat(*logFormat)
	default:
		fmt.Fprintf(stderr, "error: --log-format must be 'human' or 'json', got %q\n", *logFormat)
		return 1
	}

	// The request log goes to stderr by default. When the live view is active
	// (TTY + not --no-live), the log writer is silenced so the two streams
	// don't interleave and corrupt the live display. Users who need the log
	// can re-run with --no-live.
	logWriter := log.NewWriterWithFormat(stderr, lf)
	if !*noLive && log.IsTerminal(stderr) {
		logWriter = log.Disabled()
	}

	proxyHandler := proxy.NewHandlerWithStoreAndUsageDebug(logWriter, st, *project, usageDebug)
	proxyHandler.SetUpstream(*upstream)
	proxyHandler.SetHeadroomProxyAddr(*headroomProxyAddr)
	cat, err := st.Catalog()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load pricing catalog: %v\n", err)
		return 1
	}
	proxyHandler.SetCatalog(cat)
	proxyHandler.SetRawLogger(rawLogger)
	anomalyRecorder := proxy.NewAnomalyRecorder(st)
	defer anomalyRecorder.Shutdown()
	proxyHandler.SetAnomalyRecorder(anomalyRecorder)

	// Dashboard sidecar: start on separate port 7734 when --dashboard is set.
	var dashServer *http.Server
	if *dashboardFlag {
		dashAddr := "127.0.0.1:7734"
		dashMux := http.NewServeMux()
		dashMux.Handle("/api/", api.NewHandler(st))
		dashMux.Handle("/", dashboard.Handler())
		dashServer = &http.Server{
			Addr:              dashAddr,
			Handler:           dashMux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			if err := dashServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(stderr, "dashboard server failed: %v\n", err)
			}
		}()
		fmt.Fprintf(stdout, "dashboard: http://%s (UI)  http://%s/api/ (API)\n", dashAddr, dashAddr)
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           proxyHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))
	fmt.Fprintf(stdout, "upstream: %s\n", *upstream)
	fmt.Fprintf(stdout, "headroom proxy: %s\n", *headroomProxyAddr)
	if *usageDebugPath != "" {
		fmt.Fprintf(stdout, "usage debug log: %s\n", store.FormatPath(*usageDebugPath))
	}

	// Live session tail. Active by default; runs only when stderr is a TTY.
	// Disabled with --no-live or when the user redirected stderr to a file/pipe.
	stopTail := func() {}
	if !*noLive && log.IsTerminal(stderr) {
		stopTail = startLiveTail(stderr, st)
		fmt.Fprintf(stdout, "\nLive session tail: updating every 2s (--no-live to disable).\n")
	}
	defer stopTail()

	// Graceful shutdown on Ctrl+C: stop the server, then stop the tail.
	// SIGINT -> exit 130, SIGTERM -> exit 0.
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
		if dashServer != nil {
			_ = dashServer.Shutdown(shutdownCtx)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
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

// startLiveTail runs a goroutine that periodically prints the current live session
// to w, replacing the previous content using ANSI escape codes. Returns a stop function.
func startLiveTail(w io.Writer, st *store.Store) func() {
	stop := make(chan struct{})
	var prevLines int

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				clearTail(w, prevLines)
				return
			case <-ticker.C:
				current, costResult, err := loadLiveSession(context.Background(), st)
				if err != nil {
					continue
				}
				lines := renderLiveCompact(current, costResult)
				clearTail(w, prevLines)
				prevLines = writeLines(w, lines)
			}
		}
	}()

	return func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
}

// clearTail erases the previous tail output by moving the cursor up and clearing lines.
func clearTail(w io.Writer, n int) {
	if n <= 0 {
		return
	}
	// CSI n A: cursor up n lines
	// CSI 0 J: erase from cursor to end of screen
	fmt.Fprintf(w, "\x1b[%dA\x1b[0J", n)
}

// writeLines writes the given text and returns the number of lines it occupied.
func writeLines(w io.Writer, s string) int {
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	fmt.Fprintln(w, s)
	return count
}
