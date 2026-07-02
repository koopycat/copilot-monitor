package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func runCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path")
	aText := fs.String("a", "", "first period as YYYY-MM")
	bText := fs.String("b", "", "second period as YYYY-MM")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	aStart, aEnd, bStart, bEnd, err := parseCompareMonths(*aText, *bText, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "invalid compare periods: %v\n", err)
		return 2
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open db %q: %v\n", *dbPath, err)
		return 1
	}
	defer st.Close()

	result, err := st.CompareStats(context.Background(), aStart, aEnd, bStart, bEnd)
	if err != nil {
		fmt.Fprintf(stderr, "failed to compare periods: %v\n", err)
		return 1
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "failed to load model catalog: %v\n", err)
		return 1
	}
	if len(result.Periods) != 2 {
		fmt.Fprintf(stderr, "compare returned %d periods, want 2\n", len(result.Periods))
		return 1
	}
	printCompareRows(stdout, result.Periods[0], result.Periods[1], cat)
	return 0
}

func printCompareRows(w io.Writer, a, b store.ComparePeriod, cat catalog.Catalog) {
	aCost := costcalc.Calculate(a.Models, cat)
	bCost := costcalc.Calculate(b.Models, cat)
	rows := mergeCompareRows(aCost, bCost)

	fmt.Fprintf(w, "Comparing %s to %s\n", a.Label, b.Label)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tPERIOD A COST\tPERIOD B COST\tDELTA\tPERIOD A TOKENS\tPERIOD B TOKENS\tDELTA")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.Model,
			formatDollars(row.CostA),
			formatDollars(row.CostB),
			formatDelta(row.CostA, row.CostB),
			formatTokens(row.TokensA),
			formatTokens(row.TokensB),
			formatDelta(float64(row.TokensA), float64(row.TokensB)),
		)
	}
	fmt.Fprintf(tw, "TOTAL\t%s\t%s\t%s\t%s\t%s\t%s\n",
		formatDollars(aCost.TotalUSD),
		formatDollars(bCost.TotalUSD),
		formatDelta(aCost.TotalUSD, bCost.TotalUSD),
		formatTokens(aCost.TotalTokens),
		formatTokens(bCost.TotalTokens),
		formatDelta(float64(aCost.TotalTokens), float64(bCost.TotalTokens)),
	)
	_ = tw.Flush()
}

type compareRow struct {
	Model   string
	CostA   float64
	CostB   float64
	TokensA int
	TokensB int
}

func mergeCompareRows(a, b costcalc.Total) []compareRow {
	byModel := map[string]*compareRow{}
	add := func(model string) *compareRow {
		row := byModel[model]
		if row == nil {
			row = &compareRow{Model: model}
			byModel[model] = row
		}
		return row
	}
	for _, costRow := range a.Rows {
		row := add(costRow.Model)
		row.CostA += costRow.TotalUSD
		row.TokensA += costRow.TotalTokens
	}
	for _, costRow := range b.Rows {
		row := add(costRow.Model)
		row.CostB += costRow.TotalUSD
		row.TokensB += costRow.TotalTokens
	}
	rows := make([]compareRow, 0, len(byModel))
	for _, row := range byModel {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].TokensA + rows[i].TokensB
		right := rows[j].TokensA + rows[j].TokensB
		if left != right {
			return left > right
		}
		return rows[i].Model < rows[j].Model
	})
	return rows
}

func parseCompareMonths(aText, bText string, now time.Time) (time.Time, time.Time, time.Time, time.Time, error) {
	if aText == "" && bText == "" {
		current := startOfMonth(now)
		last := current.AddDate(0, -1, 0)
		return last, current, current, current.AddDate(0, 1, 0), nil
	}
	if aText == "" || bText == "" {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("--a and --b must be provided together")
	}
	aStart, aEnd, err := compareMonthWindow(aText)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid --a %q", aText)
	}
	bStart, bEnd, err := compareMonthWindow(bText)
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid --b %q", bText)
	}
	return aStart, aEnd, bStart, bEnd, nil
}

func compareMonthWindow(value string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 1, 0), nil
}

func startOfMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
