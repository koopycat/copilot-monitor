package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"llm-proxy/dashboard"
	"llm-proxy/internal/api"
	"llm-proxy/internal/proxy"
	"llm-proxy/internal/store"
)

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7734", "HTTP listen address")
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	routesConfig := fs.String("routes-config", "", "optional JSON file with additional route definitions")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	proxyCfg, err := proxy.LoadConfig(*routesConfig)
	if err != nil {
		fmt.Fprintf(stderr, "failed to load routes config: %v\n", err)
		return 1
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandlerWithConfig(st, proxyCfg))
	mux.Handle("/", dashboard.Handler())

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintf(stdout, "llm-proxy API listening on http://%s\n", settingsAddr(*addr))
	fmt.Fprintf(stdout, "database: %s\n", store.FormatPath(*dbPath))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}
