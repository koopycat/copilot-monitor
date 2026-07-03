package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"copilot-monitoring/internal/store"
)

func runExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	since, err := parseSince(*sinceText, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "invalid --since: %v\n", err)
		return 2
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db: %v\n", err)
		return 1
	}
	defer st.Close()

	rows, err := st.ExportRequests(context.Background(), since, time.Time{})
	if err != nil {
		fmt.Fprintf(stderr, "export failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "ts,endpoint,model,status,latency_ms,prompt_tokens,cached_input_tokens,cache_write_tokens,completion_tokens,total_tokens,project")
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%s\n",
			row.Timestamp, row.Endpoint, csvField(row.Model), row.Status, row.LatencyMS,
			row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens,
			row.CompletionTokens, row.TotalTokens, csvField(row.Project))
	}
	return 0
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
