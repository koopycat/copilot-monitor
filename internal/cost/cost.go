package cost

import (
	"strings"

	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

type Row struct {
	Model                    string  `json:"model"`
	Endpoint                 string  `json:"endpoint"`
	UpstreamHost             string  `json:"upstream_host"`
	Provider                 string  `json:"provider"`
	Requests                 int     `json:"requests"`
	PromptTokens             int     `json:"prompt_tokens"`
	CachedInputTokens        int     `json:"cached_input_tokens"`
	CacheWriteTokens         int     `json:"cache_write_tokens"`
	CompletionTokens         int     `json:"completion_tokens"`
	TotalTokens              int     `json:"total_tokens"`
	InputUSD                 float64 `json:"input_usd"`
	CachedInputUSD           float64 `json:"cached_input_usd"`
	CacheWriteUSD            float64 `json:"cache_write_usd"`
	OutputUSD                float64 `json:"output_usd"`
	TotalUSD                 float64 `json:"total_usd"`
	Fallback                 bool    `json:"fallback"`
	NotBilled                bool    `json:"not_billed"`
	CompressedRequests       int     `json:"compressed_requests"`
	CompressionRemovedTokens int     `json:"compression_removed_tokens"`
}

type Total struct {
	Rows                     []Row   `json:"rows"`
	Requests                 int     `json:"requests"`
	PromptTokens             int     `json:"prompt_tokens"`
	CachedInputTokens        int     `json:"cached_input_tokens"`
	CacheWriteTokens         int     `json:"cache_write_tokens"`
	CompletionTokens         int     `json:"completion_tokens"`
	TotalTokens              int     `json:"total_tokens"`
	InputUSD                 float64 `json:"input_usd"`
	CachedInputUSD           float64 `json:"cached_input_usd"`
	CacheWriteUSD            float64 `json:"cache_write_usd"`
	OutputUSD                float64 `json:"output_usd"`
	TotalUSD                 float64 `json:"total_usd"`
	FallbackCount            int     `json:"fallback_count"`
	NotBilledCount           int     `json:"not_billed_count"`
	CompressionRemovedTokens int     `json:"compression_removed_tokens"`
}

func Calculate(stats []store.ModelStats, catalog catalog.Catalog) Total {
	var total Total
	for _, stat := range stats {
		lookup := catalog.Lookup(stat.Model)
		// If the model isn't in the catalog and we have a provider hint,
		// try the provider-specific fallback pricing.
		pricing := lookup.Pricing
		if lookup.Fallback && stat.Provider != "" {
			if pf, ok := catalog.ProviderFallbacks[strings.ToLower(stat.Provider)]; ok {
				pricing = pf
			}
		}
		row := Row{
			Model:             stat.Model,
			Endpoint:          stat.Endpoint,
			UpstreamHost:      stat.UpstreamHost,
			Provider:          stat.Provider,
			Requests:          stat.Requests,
			PromptTokens:      stat.PromptTokens,
			CachedInputTokens: stat.CachedInputTokens,
			CacheWriteTokens:  stat.CacheWriteTokens,
			CompletionTokens:  stat.CompletionTokens,
			TotalTokens:       stat.TotalTokens,
			Fallback:          lookup.Fallback,
			NotBilled:         stat.NotBilled,
		}
		if !row.NotBilled {
			regularInputTokens := stat.PromptTokens - stat.CachedInputTokens
			if regularInputTokens < 0 {
				regularInputTokens = 0
			}
			row.InputUSD = costForTokens(regularInputTokens, pricing.InputPerM)
			row.CachedInputUSD = costForTokens(stat.CachedInputTokens, pricing.CachedInputPerM)
			row.CacheWriteUSD = costForTokens(stat.CacheWriteTokens, pricing.CacheWritePerM)
			row.OutputUSD = costForTokens(stat.CompletionTokens, pricing.OutputPerM)
			row.TotalUSD = row.InputUSD + row.CachedInputUSD + row.CacheWriteUSD + row.OutputUSD
		}
		total.Rows = append(total.Rows, row)
		total.Requests += row.Requests
		total.PromptTokens += row.PromptTokens
		total.CachedInputTokens += row.CachedInputTokens
		total.CacheWriteTokens += row.CacheWriteTokens
		total.CompletionTokens += row.CompletionTokens
		total.TotalTokens += row.TotalTokens
		total.CompressionRemovedTokens += row.CompressionRemovedTokens
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

func costForTokens(tokens int, perMillion float64) float64 {
	return float64(tokens) / 1_000_000 * perMillion
}
