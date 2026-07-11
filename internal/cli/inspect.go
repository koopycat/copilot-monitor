package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"copilot-monitoring/internal/store"
)

// validAnomalyCategories maps known anomaly categories to whether they are valid.
var validAnomalyCategories = map[string]bool{
	"unrouted_path":        true,
	"parse_error":          true,
	"auth_missing":         true,
	"unknown_content_type": true,
	"unknown_upstream":     true,
	"unknown_ws_event":     true,
}

// severityOrder maps severity strings to sort order (higher = more severe).
var severityOrder = map[string]int{
	"error": 3,
	"warn":  2,
	"info":  1,
}

func runInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	sinceText := fs.String("since", "24h", "duration to look back, e.g. 1h, 24h, 7d")
	category := fs.String("category", "", "filter by anomaly category (unrouted_path, parse_error, auth_missing, unknown_content_type, unknown_upstream, unknown_ws_event)")
	severity := fs.String("severity", "", "filter by severity (info, warn, error)")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	alertOnAny := fs.Bool("alert-on-any", false, "exit 1 if any anomalies match")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *category != "" && !validAnomalyCategories[*category] {
		fmt.Fprintf(stderr, "invalid --category %q: valid values are %s\n", *category, strings.Join(categoryNames(), ", "))
		return 2
	}

	if *severity != "" && *severity != "info" && *severity != "warn" && *severity != "error" {
		fmt.Fprintf(stderr, "invalid --severity %q: valid values are info, warn, error\n", *severity)
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

	anomalies, err := st.QueryAnomalies(context.Background(), store.AnomalyFilter{
		Since:    since,
		Category: *category,
		Severity: *severity,
	})
	if err != nil {
		fmt.Fprintf(stderr, "failed to query anomalies: %v\n", err)
		return 1
	}

	if *jsonFlag {
		return printAnomaliesJSON(stdout, anomalies, *alertOnAny)
	}
	return printAnomaliesTable(stdout, anomalies, *sinceText, *alertOnAny)
}

// categoryGroup holds aggregated info for one (severity, category) pair.
type categoryGroup struct {
	severity string
	category string
	count    int
	latestTS string
	sample   string
}

func printAnomaliesTable(stdout io.Writer, anomalies []store.Anomaly, sinceText string, alertOnAny bool) int {
	if len(anomalies) == 0 {
		fmt.Fprintf(stdout, "No anomalies found in the last %s.\n", sinceText)
		return 0
	}

	// Group by severity + category
	groups := make(map[string]*categoryGroup)
	for _, a := range anomalies {
		key := a.Severity + "|" + a.Category
		if g, ok := groups[key]; ok {
			g.count++
			if a.TS > g.latestTS {
				g.latestTS = a.TS
				g.sample = a.Detail
			}
		} else {
			groups[key] = &categoryGroup{
				severity: a.Severity,
				category: a.Category,
				count:    1,
				latestTS: a.TS,
				sample:   a.Detail,
			}
		}
	}

	// Sort groups by severity desc, then category asc
	sorted := make([]*categoryGroup, 0, len(groups))
	for _, g := range groups {
		sorted = append(sorted, g)
	}
	sort.Slice(sorted, func(i, j int) bool {
		si, sj := severityOrder[sorted[i].severity], severityOrder[sorted[j].severity]
		if si != sj {
			return si > sj
		}
		return sorted[i].category < sorted[j].category
	})

	fmt.Fprintf(stdout, "ANOMALIES (last %s)\n", sinceText)
	for _, g := range sorted {
		sevLabel := strings.ToUpper(g.severity)
		ts := truncateTimestamp(g.latestTS)
		fmt.Fprintf(stdout, "  %-20s %4d   %-5s  %s  %s\n",
			g.category, g.count, sevLabel, ts, g.sample)
	}
	fmt.Fprintf(stdout, "\n%d total anomaly records\n", len(anomalies))

	if alertOnAny && len(anomalies) > 0 {
		return 1
	}
	return 0
}

func printAnomaliesJSON(stdout io.Writer, anomalies []store.Anomaly, alertOnAny bool) int {
	if anomalies == nil {
		anomalies = []store.Anomaly{}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(anomalies)
	if alertOnAny && len(anomalies) > 0 {
		return 1
	}
	return 0
}

func truncateTimestamp(ts string) string {
	if len(ts) >= 19 {
		return ts[:19]
	}
	return ts
}

func categoryNames() []string {
	names := make([]string, 0, len(validAnomalyCategories))
	for n := range validAnomalyCategories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
