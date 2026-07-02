package cost

import (
	"math"
	"testing"

	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

func TestCalculateWithCachedAndCacheWriteTokens(t *testing.T) {
	catalog, err := catalog.Load([]byte(`{
		"currency":"USD",
		"fallback":{"provider":"unknown","input_per_m":5,"cached_input_per_m":0.5,"cache_write_per_m":5,"output_per_m":15},
		"provider_fallbacks":{"openai":{"provider":"openai-fallback","input_per_m":2,"cached_input_per_m":0.2,"cache_write_per_m":3,"output_per_m":8}},
		"models":{"gpt-5-mini":{"provider":"openai","input_per_m":2,"cached_input_per_m":0.2,"cache_write_per_m":3,"output_per_m":8}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Calculate([]store.ModelStats{{
		Model:             "gpt-5-mini",
		Endpoint:          "chat",
		Requests:          2,
		PromptTokens:      1_000_000,
		CachedInputTokens: 250_000,
		CacheWriteTokens:  100_000,
		CompletionTokens:  500_000,
		TotalTokens:       1_600_000,
	}}, catalog)

	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(got.Rows))
	}
	row := got.Rows[0]
	if !closeEnough(row.InputUSD, 1.5) || !closeEnough(row.CachedInputUSD, 0.05) || !closeEnough(row.CacheWriteUSD, 0.3) || !closeEnough(row.OutputUSD, 4) || !closeEnough(row.TotalUSD, 5.85) {
		t.Fatalf("row costs = %#v", row)
	}
	if !closeEnough(got.TotalUSD, 5.85) || got.Requests != 2 || got.TotalTokens != 1_600_000 {
		t.Fatalf("total = %#v", got)
	}
}

func TestCalculateProviderFallback(t *testing.T) {
	catalog, err := catalog.Load([]byte(`{
		"currency":"USD",
		"fallback":{"provider":"unknown","input_per_m":5,"cached_input_per_m":0.5,"cache_write_per_m":5,"output_per_m":15},
		"provider_fallbacks":{"anthropic":{"provider":"anthropic-fallback","input_per_m":3,"cached_input_per_m":0.3,"cache_write_per_m":3.75,"output_per_m":15}},
		"models":{"known":{"provider":"x","input_per_m":1,"cached_input_per_m":0.1,"cache_write_per_m":1,"output_per_m":1}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Calculate([]store.ModelStats{{
		Model:            "claude-new-model",
		Endpoint:         "chat",
		Requests:         1,
		PromptTokens:     1_000_000,
		CompletionTokens: 1_000_000,
		TotalTokens:      2_000_000,
	}}, catalog)

	if !got.Rows[0].Fallback {
		t.Fatal("expected fallback row")
	}
	if got.Rows[0].Provider != "anthropic-fallback" {
		t.Fatalf("provider = %q", got.Rows[0].Provider)
	}
	if got.Rows[0].TotalUSD != 18 {
		t.Fatalf("total usd = %f, want 18", got.Rows[0].TotalUSD)
	}
	if got.FallbackCount != 1 {
		t.Fatalf("fallback count = %d", got.FallbackCount)
	}
}

func TestCalculateCodeCompletionsAreNotBilled(t *testing.T) {
	catalog, err := catalog.Load([]byte(`{
		"currency":"USD",
		"fallback":{"provider":"unknown","input_per_m":5,"cached_input_per_m":0.5,"cache_write_per_m":5,"output_per_m":15},
		"provider_fallbacks":{"openai":{"provider":"openai-fallback","input_per_m":2,"cached_input_per_m":0.2,"cache_write_per_m":3,"output_per_m":8}},
		"models":{"gpt-5-mini":{"provider":"openai","input_per_m":2,"cached_input_per_m":0.2,"cache_write_per_m":3,"output_per_m":8}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Calculate([]store.ModelStats{{
		Model:            "gpt-5-mini",
		Endpoint:         "completions",
		Requests:         1,
		PromptTokens:     1_000_000,
		CompletionTokens: 1_000_000,
		TotalTokens:      2_000_000,
	}}, catalog)
	if !got.Rows[0].NotBilled || got.Rows[0].TotalUSD != 0 || got.NotBilledCount != 1 {
		t.Fatalf("row = %#v total = %#v", got.Rows[0], got)
	}
}

func closeEnough(a, b float64) bool {
	return math.Abs(a-b) < 0.0000001
}
