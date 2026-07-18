package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"copilot-monitoring/internal/store"
)

func runExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back")
	project := fs.String("project", "", "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	output := fs.String("output", "", "write output to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	since, err := parseSince(*sinceText, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "error: parsing --since %q: %v\n", *sinceText, err)
		return 2
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening database %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	rows, err := st.ExportRequests(context.Background(), since, time.Time{}, *project, *endpoint, "")
	if err != nil {
		fmt.Fprintf(stderr, "error: exporting requests: %v\n", err)
		return 1
	}

	var w io.Writer = stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(stderr, "error: creating output file %q: %v\n", *output, err)
			return 1
		}
		defer f.Close()
		w = f
	}

	fmt.Fprintln(w, "ts,endpoint,endpoint_kind,model,status,latency_ms,input_tokens,cached_input_tokens,cache_write_tokens,output_tokens,total_tokens,project,headroom_proxied")
	for _, row := range rows {
		fmt.Fprintf(w, "%s,%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%s,%t\n",
			row.Timestamp, row.Endpoint, csvField(row.EndpointKind), csvField(row.Model), row.Status, row.LatencyMS,
			row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens,
			row.CompletionTokens, row.TotalTokens, csvField(row.Project),
			row.HeadroomProxied)
	}
	if *output != "" {
		fmt.Fprintf(stdout, "exported %d rows to %s\n", len(rows), *output)
	}
	return 0
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
