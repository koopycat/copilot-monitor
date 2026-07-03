package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
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

	current, costResult, err := loadLiveSession(context.Background(), st)
	if err != nil {
		fmt.Fprintf(stderr, "failed to query live session: %v\n", err)
		return 1
	}
	if current == nil {
		fmt.Fprintf(stdout, "No sessions captured yet.\n")
		return 0
	}

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

	renderLive(stdout, current, costResult)
	return 0
}

// loadLiveSession rebuilds sessions, fetches the current one, and calculates its cost.
// Returns nil current when no sessions have been captured yet.
func loadLiveSession(ctx context.Context, st *store.Store) (*store.CurrentSession, costcalc.Total, error) {
	if err := st.RebuildSessions(ctx, 30*time.Minute); err != nil {
		return nil, costcalc.Total{}, err
	}
	current, err := st.CurrentSession(ctx)
	if err != nil {
		return nil, costcalc.Total{}, err
	}
	if current == nil {
		return nil, costcalc.Total{}, nil
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		return nil, costcalc.Total{}, err
	}
	return current, costcalc.Calculate(current.Models, cat), nil
}

// renderLive writes the current session in the canonical full format.
func renderLive(w io.Writer, current *store.CurrentSession, costResult costcalc.Total) {
	end := current.LastRequestAt
	if current.Active {
		end = time.Now()
	}
	duration := end.Sub(current.StartedAt).Round(time.Second)
	project := current.Project
	if project == "" {
		project = "(mixed)"
	}

	fmt.Fprintf(w, "Live session  %s\n", liveStatus(current))
	fmt.Fprintf(w, "  started     %s\n", current.StartedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "  last        %s\n", current.LastRequestAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "  duration    %s\n", duration)
	fmt.Fprintf(w, "  project     %s\n", project)
	fmt.Fprintf(w, "  requests    %d\n", current.RequestCount)
	fmt.Fprintf(w, "  tokens      %s\n", intComma(current.TokenCount))
	fmt.Fprintf(w, "  est. cost   %s\n", formatUSD(costResult.TotalUSD))

	if len(costResult.Rows) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
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
		fmt.Fprintf(w, "\n* provider or generic fallback pricing used for %d row(s). Code-completion rows are not billed in AI credits.\n", costResult.FallbackCount)
	}
}

// renderLiveCompact writes a 3-line summary suitable for periodic refresh in a terminal.
// Returns the rendered text without a trailing newline.
func renderLiveCompact(current *store.CurrentSession, costResult costcalc.Total) string {
	if current == nil {
		return "Live session  ○ no sessions captured yet"
	}
	duration := time.Since(current.StartedAt).Round(time.Second)
	if !current.Active {
		duration = current.LastRequestAt.Sub(current.StartedAt).Round(time.Second)
	}
	project := current.Project
	if project == "" {
		project = "(mixed)"
	}
	top := topModels(costResult.Rows, 3)
	header := fmt.Sprintf("Live session  %s   %s   %d req   %s tok   %s   project %s",
		liveStatus(current),
		duration,
		current.RequestCount,
		intComma(current.TokenCount),
		formatUSD(costResult.TotalUSD),
		project,
	)
	models := "  " + strings.Join(top, ", ")
	return strings.TrimRight(header+"\n"+models, "\n")
}

func topModels(rows []costcalc.Row, n int) []string {
	if len(rows) == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for _, r := range rows {
		if r.Requests == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("%s %d", r.Model, r.Requests))
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
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
