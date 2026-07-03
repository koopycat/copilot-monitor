package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func runLive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	if err := st.RebuildSessions(context.Background(), 30*time.Minute); err != nil {
		fmt.Fprintf(stderr, "failed to rebuild sessions: %v\n", err)
		return 1
	}

	current, err := st.CurrentSession(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "failed to query live session: %v\n", err)
		return 1
	}
	if current == nil {
		fmt.Fprintf(stdout, "No sessions captured yet.\n")
		return 0
	}

	cat, err := catalog.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load model catalog: %v\n", err)
		return 1
	}
	costResult := costcalc.Calculate(current.Models, cat)

	if *jsonFlag {
		type liveSessionJSON struct {
			ID            int64          `json:"id"`
			StartedAt     time.Time      `json:"started_at"`
			LastRequestAt time.Time      `json:"last_request_at"`
			Project       string         `json:"project"`
			RequestCount  int            `json:"request_count"`
			TokenCount    int            `json:"token_count"`
			Cost          float64        `json:"cost"`
			Status        string         `json:"status"`
			Active        bool           `json:"active"`
			Models        []costcalc.Row `json:"models"`
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(liveSessionJSON{
			ID:            current.ID,
			StartedAt:     current.StartedAt,
			LastRequestAt: current.LastRequestAt,
			Project:       current.Project,
			RequestCount:  current.RequestCount,
			TokenCount:    current.TokenCount,
			Cost:          costResult.TotalUSD,
			Status:        current.Status,
			Active:        current.Active,
			Models:        costResult.Rows,
		}); err != nil {
			fmt.Fprintf(stderr, "json encode failed: %v\n", err)
			return 1
		}
		return 0
	}

	end := current.LastRequestAt
	if current.Active {
		end = time.Now()
	}
	duration := end.Sub(current.StartedAt).Round(time.Second)
	project := current.Project
	if project == "" {
		project = "(mixed)"
	}

	fmt.Fprintf(stdout, "Live session  %s\n", liveStatus(current))
	fmt.Fprintf(stdout, "  started     %s\n", current.StartedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(stdout, "  last        %s\n", current.LastRequestAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(stdout, "  duration    %s\n", duration)
	fmt.Fprintf(stdout, "  project     %s\n", project)
	fmt.Fprintf(stdout, "  requests    %d\n", current.RequestCount)
	fmt.Fprintf(stdout, "  tokens      %s\n", intComma(current.TokenCount))
	fmt.Fprintf(stdout, "  est. cost   %s\n", formatUSD(costResult.TotalUSD))

	if len(costResult.Rows) == 0 {
		return 0
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nMODEL\tENDPOINT\tREQUESTS\tINPUT\tCACHED\tOUTPUT\tCOST")
	for _, row := range costResult.Rows {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			row.Model,
			row.Endpoint,
			row.Requests,
			intComma(row.PromptTokens),
			intComma(row.CachedInputTokens),
			intComma(row.CompletionTokens),
			formatUSD(row.TotalUSD),
		)
	}
	_ = tw.Flush()

	if costResult.FallbackCount > 0 || costResult.NotBilledCount > 0 {
		fmt.Fprintf(stdout, "\n* provider or generic fallback pricing used for %d row(s). Code-completion rows are not billed in AI credits.\n", costResult.FallbackCount)
	}
	return 0
}

func liveStatus(current *store.CurrentSession) string {
	if current.Active {
		return "● active"
	}
	return fmt.Sprintf("○ idle %s", formatAgo(time.Since(current.LastRequestAt)))
}

func formatAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func formatUSD(v float64) string {
	if v < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", v)
}

func intComma(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	// simple grouping with commas
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
