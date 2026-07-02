package cost

import (
	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

type Row struct {
	Model             string
	Endpoint          string
	Provider          string
	Requests          int
	PromptTokens      int
	CachedInputTokens int
	CacheWriteTokens  int
	CompletionTokens  int
	TotalTokens       int
	InputUSD          float64
	CachedInputUSD    float64
	CacheWriteUSD     float64
	OutputUSD         float64
	TotalUSD          float64
	Fallback          bool
	NotBilled         bool
}

type Total struct {
	Rows              []Row
	Requests          int
	PromptTokens      int
	CachedInputTokens int
	CacheWriteTokens  int
	CompletionTokens  int
	TotalTokens       int
	InputUSD          float64
	CachedInputUSD    float64
	CacheWriteUSD     float64
	OutputUSD         float64
	TotalUSD          float64
	FallbackCount     int
	NotBilledCount    int
}

func Calculate(stats []store.ModelStats, catalog catalog.Catalog) Total {
	var total Total
	for _, stat := range stats {
		lookup := catalog.Lookup(stat.Model)
		row := Row{
			Model:             stat.Model,
			Endpoint:          stat.Endpoint,
			Provider:          lookup.Pricing.Provider,
			Requests:          stat.Requests,
			PromptTokens:      stat.PromptTokens,
			CachedInputTokens: stat.CachedInputTokens,
			CacheWriteTokens:  stat.CacheWriteTokens,
			CompletionTokens:  stat.CompletionTokens,
			TotalTokens:       stat.TotalTokens,
			Fallback:          lookup.Fallback,
			NotBilled:         isNotBilledEndpoint(stat.Endpoint),
		}
		if !row.NotBilled {
			regularInputTokens := stat.PromptTokens - stat.CachedInputTokens
			if regularInputTokens < 0 {
				regularInputTokens = 0
			}
			row.InputUSD = costForTokens(regularInputTokens, lookup.Pricing.InputPerM)
			row.CachedInputUSD = costForTokens(stat.CachedInputTokens, lookup.Pricing.CachedInputPerM)
			row.CacheWriteUSD = costForTokens(stat.CacheWriteTokens, lookup.Pricing.CacheWritePerM)
			row.OutputUSD = costForTokens(stat.CompletionTokens, lookup.Pricing.OutputPerM)
			row.TotalUSD = row.InputUSD + row.CachedInputUSD + row.CacheWriteUSD + row.OutputUSD
		}
		total.Rows = append(total.Rows, row)
		total.Requests += row.Requests
		total.PromptTokens += row.PromptTokens
		total.CachedInputTokens += row.CachedInputTokens
		total.CacheWriteTokens += row.CacheWriteTokens
		total.CompletionTokens += row.CompletionTokens
		total.TotalTokens += row.TotalTokens
		total.InputUSD += row.InputUSD
		total.CachedInputUSD += row.CachedInputUSD
		total.CacheWriteUSD += row.CacheWriteUSD
		total.OutputUSD += row.OutputUSD
		total.TotalUSD += row.TotalUSD
		if row.Fallback {
			total.FallbackCount++
		}
		if row.NotBilled {
			total.NotBilledCount++
		}
	}
	return total
}

func isNotBilledEndpoint(endpoint string) bool {
	return endpoint == "completions"
}

func costForTokens(tokens int, perMillion float64) float64 {
	return float64(tokens) / 1_000_000 * perMillion
}
