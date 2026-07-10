package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"llm-proxy/internal/catalog"
	costcalc "llm-proxy/internal/cost"
	"llm-proxy/internal/store"
)

func runCost(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "30d", "duration to look back, e.g. 24h, 7d, 30d, or all")
	project := fs.String("project", "", "filter by project")
	endpoint := fs.String("endpoint", "", "filter by endpoint")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	since, err := parseSince(*sinceText, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "invalid --since value %q: %v\n", *sinceText, err)
		return 2
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	rows, err := st.Stats(context.Background(), store.StatsFilter{Since: since, Project: *project, Endpoint: *endpoint})
	if err != nil {
		fmt.Fprintf(stderr, "failed to query stats: %v\n", err)
		return 1
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load model catalog: %v\n", err)
		return 1
	}
	total := costcalc.Calculate(rows, cat)

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(total); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Estimated equivalent LLM API list-price cost (%s). This is not your actual bill.\n", cat.Currency)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tENDPOINT\tPROVIDER\tREQUESTS\tINPUT_TOK\tCACHED_TOK\tCACHE_WRITE_TOK\tOUTPUT_TOK\tINPUT $\tCACHED $\tCACHE WRITE $\tOUTPUT $\tEST. LIST $")
	for _, row := range total.Rows {
		provider := row.Provider
		if row.Fallback {
			provider += "*"
		}
		if row.NotBilled {
			provider += " (not billed)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%.6f\t%.6f\t%.6f\t%.6f\t%.6f\n", row.Model, row.Endpoint, provider, row.Requests, row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens, row.CompletionTokens, row.InputUSD, row.CachedInputUSD, row.CacheWriteUSD, row.OutputUSD, row.TotalUSD)
	}
	fmt.Fprintf(tw, "TOTAL\t\t\t%d\t%d\t%d\t%d\t%d\t%.6f\t%.6f\t%.6f\t%.6f\t%.6f\n", total.Requests, total.PromptTokens, total.CachedInputTokens, total.CacheWriteTokens, total.CompletionTokens, total.InputUSD, total.CachedInputUSD, total.CacheWriteUSD, total.OutputUSD, total.TotalUSD)
	_ = tw.Flush()
	if total.FallbackCount > 0 {
		fmt.Fprintf(stdout, "\n* provider or generic fallback pricing used for %d row(s).\n", total.FallbackCount)
	}
	if total.NotBilledCount > 0 {
		fmt.Fprintf(stdout, "Inline code completion rows are shown with zero AI-credit cost because GitHub docs say code completions are not billed in AI credits.\n")
	}
	return 0
}
