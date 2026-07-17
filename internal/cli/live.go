package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func runLive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	watchFlag := fs.Bool("watch", false, "refresh every 2s (Ctrl+C to stop)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	if *watchFlag {
		return runLiveWatch(stdout, st)
	}

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

// loadLiveSession fetches the incrementally-maintained current session and calculates its cost.
// Returns nil current when no sessions have been captured yet.
func loadLiveSession(ctx context.Context, st *store.Store) (*store.CurrentSession, costcalc.Total, error) {
	current, err := st.CurrentSession(ctx)
	if err != nil {
		return nil, costcalc.Total{}, err
	}
	if current == nil {
		return nil, costcalc.Total{}, nil
	}
	cat, err := st.Catalog()
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
	fmt.Fprintf(w, "  est. rate   %s\n", formatUSD(costResult.TotalUSD))

	if len(costResult.Rows) == 0 {
		return
	}

	overview := aggregateByModel(costResult.Rows)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nMODEL\tREQUESTS\tINPUT\tCACHED\tCACHE HIT\tOUTPUT\tCOST")
	for _, m := range overview {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			m.model,
			m.requests,
			intComma(m.input),
			intComma(m.cached),
			m.cacheHit(),
			intComma(m.output),
			formatUSD(m.cost),
		)
	}
	if len(overview) > 1 {
		totals := modelTotals(overview)
		fmt.Fprintf(tw, "TOTAL\t%d\t%s\t%s\t%s\t%s\t%s\n",
			totals.requests,
			intComma(totals.input),
			intComma(totals.cached),
			totals.cacheHit(),
			intComma(totals.output),
			formatUSD(totals.cost),
		)
	}
	_ = tw.Flush()

	if costResult.CompressionRemovedTokens > 0 {
		fmt.Fprintf(w, "\nCompression: %s tokens removed\n", intComma(costResult.CompressionRemovedTokens))
	}
	if costResult.FallbackCount > 0 || costResult.NotBilledCount > 0 {
		if costResult.FallbackCount > 0 {
			fmt.Fprintf(w, "\n* provider or generic fallback pricing used for %d row(s).\n", costResult.FallbackCount)
		}
		if costResult.NotBilledCount > 0 {
			fmt.Fprintf(w, "%d row(s) are marked not billed and excluded from the estimate.\n", costResult.NotBilledCount)
		}
	}
}

// modelSummary is a per-model rollup of cost rows.
type modelSummary struct {
	model    string
	requests int
	input    int
	cached   int
	output   int
	cost     float64
}

func (m modelSummary) cacheHit() string {
	total := m.input + m.cached
	if total == 0 {
		return "—"
	}
	pct := float64(m.cached) / float64(total) * 100
	return fmt.Sprintf("%.0f%%", pct)
}

// aggregateByModel collapses per-endpoint rows into per-model rows, sorted by cost desc.
func aggregateByModel(rows []costcalc.Row) []modelSummary {
	byModel := make(map[string]*modelSummary)
	order := []string{}
	for _, r := range rows {
		if m, ok := byModel[r.Model]; ok {
			m.requests += r.Requests
			m.input += r.PromptTokens
			m.cached += r.CachedInputTokens
			m.output += r.CompletionTokens
			m.cost += r.TotalUSD
		} else {
			byModel[r.Model] = &modelSummary{
				model:    r.Model,
				requests: r.Requests,
				input:    r.PromptTokens,
				cached:   r.CachedInputTokens,
				output:   r.CompletionTokens,
				cost:     r.TotalUSD,
			}
			order = append(order, r.Model)
		}
	}
	out := make([]modelSummary, 0, len(byModel))
	for _, model := range order {
		out = append(out, *byModel[model])
	}
	// sort by cost desc, stable to keep insertion order on ties
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].cost > out[j-1].cost; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func modelTotals(rows []modelSummary) modelSummary {
	var t modelSummary
	for _, m := range rows {
		t.requests += m.requests
		t.input += m.input
		t.cached += m.cached
		t.output += m.output
		t.cost += m.cost
	}
	return t
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
	header := fmt.Sprintf("Live session  %s   %s   %d req   %s tok   %s   project %s",
		liveStatus(current),
		duration,
		current.RequestCount,
		intComma(current.TokenCount),
		formatUSD(costResult.TotalUSD),
		project,
	)
	if costResult.CompressionRemovedTokens > 0 {
		header += fmt.Sprintf("   compress -%s tok", intComma(costResult.CompressionRemovedTokens))
	}
	overview := aggregateByModel(costResult.Rows)
	top := topModelSummaries(overview, 3)
	models := "  " + strings.Join(top, ", ")
	return strings.TrimRight(header+"\n"+models, "\n")
}

func topModelSummaries(models []modelSummary, n int) []string {
	out := make([]string, 0, n)
	for _, m := range models {
		if m.requests == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("%s %d", m.model, m.requests))
		if len(out) >= n {
			break
		}
	}
	return out
}

// runLiveWatch continuously refreshes the full live view, clearing the screen before each render.
func runLiveWatch(w io.Writer, st *store.Store) int {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		// Clear whole screen, move to home, render.
		fmt.Fprint(w, "\x1b[2J\x1b[H")

		current, costResult, err := loadLiveSession(context.Background(), st)
		if err != nil {
			// Sleep briefly then retry.
			select {
			case <-sigCh:
				return 0
			case <-ticker.C:
				continue
			}
		}
		if current == nil {
			fmt.Fprintf(w, "No sessions captured yet.\n")
			fmt.Fprintf(w, "Ctrl+C to stop\n")
		} else {
			renderLive(w, current, costResult)
			fmt.Fprintf(w, "\nCtrl+C to stop")
		}

		select {
		case <-sigCh:
			return 0
		case <-ticker.C:
		}
	}
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
