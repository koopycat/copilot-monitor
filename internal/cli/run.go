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

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"
)

func runServer(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7733", "HTTP listen address")
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	project := fs.String("project", "", "optional project label")
	usageDebugPath := fs.String("usage-debug-log", "", "optional JSONL path for usage-only debug metadata")
	noLive := fs.Bool("no-live", false, "disable the live session tail below the startup banner")
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

	// The request log goes to stderr by default. When the live view is active
	// (TTY + not --no-live), the log writer is silenced so the two streams
	// don't interleave and corrupt the live display. Users who need the log
	// can re-run with --no-live.
	logWriter := log.NewWriter(stderr)
	if !*noLive && log.IsTerminal(stderr) {
		logWriter = log.Disabled()
	}

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

	// Live session tail. Active by default; runs only when stderr is a TTY.
	// Disabled with --no-live or when the user redirected stderr to a file/pipe.
	stopTail := func() {}
	if !*noLive && log.IsTerminal(stderr) {
		stopTail = startLiveTail(stderr, st)
		fmt.Fprintf(stdout, "\nLive session tail: updating every 2s (--no-live to disable).\n")
	}
	defer stopTail()

	// Graceful shutdown on Ctrl+C: stop the server, then stop the tail.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
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
