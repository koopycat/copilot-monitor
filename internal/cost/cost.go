package cost

import (
	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

type Row struct {
	Model            string
	Endpoint         string
	Provider         string
	Requests         int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	InputUSD         float64
	OutputUSD        float64
	TotalUSD         float64
	Fallback         bool
}

type Total struct {
	Rows             []Row
	Requests         int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	InputUSD         float64
	OutputUSD        float64
	TotalUSD         float64
	FallbackCount    int
}

func Calculate(stats []store.ModelStats, catalog catalog.Catalog) Total {
	var total Total
	for _, stat := range stats {
		lookup := catalog.Lookup(stat.Model)
		input := costForTokens(stat.PromptTokens, lookup.Pricing.InputPerM)
		output := costForTokens(stat.CompletionTokens, lookup.Pricing.OutputPerM)
		row := Row{
			Model:            stat.Model,
			Endpoint:         stat.Endpoint,
			Provider:         lookup.Pricing.Provider,
			Requests:         stat.Requests,
			PromptTokens:     stat.PromptTokens,
			CompletionTokens: stat.CompletionTokens,
			TotalTokens:      stat.TotalTokens,
			InputUSD:         input,
			OutputUSD:        output,
			TotalUSD:         input + output,
			Fallback:         lookup.Fallback,
		}
		total.Rows = append(total.Rows, row)
		total.Requests += row.Requests
		total.PromptTokens += row.PromptTokens
		total.CompletionTokens += row.CompletionTokens
		total.TotalTokens += row.TotalTokens
		total.InputUSD += row.InputUSD
		total.OutputUSD += row.OutputUSD
		total.TotalUSD += row.TotalUSD
		if row.Fallback {
			total.FallbackCount++
		}
	}
	return total
}

func costForTokens(tokens int, perMillion float64) float64 {
	return float64(tokens) / 1_000_000 * perMillion
}
