package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"copilot-monitoring/dashboard"
	"copilot-monitoring/internal/api"
	"copilot-monitoring/internal/catalog"
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
	rawLogPath := fs.String("raw-log", "", "optional JSONL path for raw request debugging (logs truncated bodies, headers; treat output as sensitive)")
	routesConfig := fs.String("routes-config", "", "JSON file with route definitions (defaults to built-in routes)")
	routesConfigDefaults := fs.Bool("routes-config-defaults", false, "print built-in default routes as JSON and exit")
	noLive := fs.Bool("no-live", false, "disable the live session tail below the startup banner")
	dashboardFlag := fs.Bool("dashboard", false, "also serve the dashboard API and UI on the same port (no need for a separate serve command)")
	logFormat := fs.String("log-format", "human", "log output format: human (rich colored output, default) or json (one JSON object per line)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *routesConfigDefaults {
		data, err := json.MarshalIndent(proxy.DefaultRoutes(), "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "failed to marshal default routes: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
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

	rawLogger, err := proxy.OpenRawLogger(*rawLogPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open raw debug log %q: %v\n", *rawLogPath, err)
		return 1
	}
	defer rawLogger.Close()

	if *rawLogPath != "" {
		fmt.Fprintf(stderr, "raw debug logging is enabled: request bodies (up to %d bytes) are written to %s. This file may contain source code and prompts. Treat it as sensitive.\n", 1024, *rawLogPath)
	}

	var proxyCfg *proxy.ProxyConfig
	var sourceTag string
	if *routesConfig == "" {
		defaultPath := defaultConfigPath()
		cfg, err := proxy.LoadConfig(defaultPath)
		if cfg != nil && err == nil && len(cfg.Routes) > 0 {
			proxyCfg = cfg
			*routesConfig = defaultPath
			sourceTag = fmt.Sprintf("(%d routes from %s)", len(proxyCfg.Routes), defaultPath)
		} else {
			if err != nil {
				fmt.Fprintf(stderr, "warning: default routes config %s is invalid (%v), using built-in defaults\n", defaultPath, err)
			} else if cfg != nil && len(cfg.Routes) == 0 {
				fmt.Fprintf(stderr, "warning: default routes config %s contains no routes, using built-in defaults\n", defaultPath)
			}
			proxyCfg = proxy.DefaultRoutes()
			sourceTag = "(built-in default routes)"
		}
	} else {
		var err error
		proxyCfg, err = proxy.LoadConfig(*routesConfig)
		if err != nil {
			fmt.Fprintf(stderr, "failed to load routes config %q: %v\n", *routesConfig, err)
			return 1
		}
		if len(proxyCfg.Routes) == 0 {
			fmt.Fprintf(stderr, "error: routes config %q contains no routes\n", *routesConfig)
			return 1
		}
		sourceTag = fmt.Sprintf("(%d routes from %s)", len(proxyCfg.Routes), *routesConfig)
	}

	// First-line startup banner — must appear before any other output.
	fmt.Fprintf(stderr, "copilot-monitor: listening on %s %s - curl http://%s/_ping\n", settingsAddr(*addr), sourceTag, settingsAddr(*addr))

	router := proxy.NewRouter(proxyCfg)

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

	proxyHandler := proxy.NewHandlerWithRouter(logWriter, st, *project, usageDebug, router)
	cat, err := catalog.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load pricing catalog: %v\n", err)
		return 1
	}
	proxyHandler.SetCatalog(cat)
	proxyHandler.SetRawLogger(rawLogger)
	anomalyRecorder := proxy.NewAnomalyRecorder(st)
	defer anomalyRecorder.Shutdown()
	proxyHandler.SetAnomalyRecorder(anomalyRecorder)

	var serverHandler http.Handler = proxyHandler
	if *dashboardFlag {
		serverHandler = combinedDashProxy(proxyHandler, router, api.NewHandler(st), dashboard.Handler())
		fmt.Fprintf(stdout, "dashboard: http://%s (UI)  http://%s/api/ (API)\n", settingsAddr(*addr), settingsAddr(*addr))
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           serverHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "copilot-monitor listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))
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
	// SIGINT → exit 130, SIGTERM → exit 0.
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

// combinedDashProxy returns a handler that serves the dashboard/API and proxied
// LLM traffic on the same port. LLM paths recognized by the proxy router are
// forwarded; everything else falls through to the dashboard (static files at /,
// API at /api/).
func combinedDashProxy(proxyHandler http.Handler, router *proxy.Router, apiHandler http.Handler, dashHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := router.Match(r.URL.Path); ok {
			proxyHandler.ServeHTTP(w, r)
			return
		}
		// Redirect /api (no trailing slash) to /api/ so the API mux picks it up.
		if strings.TrimRight(r.URL.Path, "/") == "/api" {
			http.Redirect(w, r, "/api/", http.StatusMovedPermanently)
			return
		}
		// Only serve the dashboard SPA for browser requests (accepts HTML).
		// API/proxy requests that don't match a route go to the proxy handler
		// which returns 502 for unknown paths.
		if acceptsHTML(r) {
			dashHandler.ServeHTTP(w, r)
			return
		}
		proxyHandler.ServeHTTP(w, r)
	}))
	return mux
}

// acceptsHTML returns true if the request's Accept header prefers text/html.
func acceptsHTML(r *http.Request) bool {
	for _, a := range strings.Split(r.Header.Get("Accept"), ",") {
		if strings.HasPrefix(strings.TrimSpace(a), "text/html") {
			return true
		}
	}
	return false
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

// defaultConfigPath returns the default routes config path at
// $XDG_CONFIG_HOME/copilot-monitor/routes.json, falling back to
// ~/.config/copilot-monitor/routes.json.
func defaultConfigPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "copilot-monitor", "routes.json")
}
