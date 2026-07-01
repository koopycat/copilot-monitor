package cost

import (
	"testing"

	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

func TestCalculate(t *testing.T) {
	catalog, err := catalog.Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"gpt-4o":{"provider":"openai","input_per_m":2,"output_per_m":8}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Calculate([]store.ModelStats{{
		Model:            "gpt-4o",
		Endpoint:         "chat",
		Requests:         2,
		PromptTokens:     1_000_000,
		CompletionTokens: 500_000,
		TotalTokens:      1_500_000,
	}}, catalog)

	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(got.Rows))
	}
	row := got.Rows[0]
	if row.InputUSD != 2 || row.OutputUSD != 4 || row.TotalUSD != 6 {
		t.Fatalf("row costs = %#v", row)
	}
	if got.TotalUSD != 6 || got.Requests != 2 || got.TotalTokens != 1_500_000 {
		t.Fatalf("total = %#v", got)
	}
}

func TestCalculateFallback(t *testing.T) {
	catalog, err := catalog.Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"known":{"provider":"x","input_per_m":1,"output_per_m":1}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := Calculate([]store.ModelStats{{
		Model:            "unknown",
		Endpoint:         "chat",
		Requests:         1,
		PromptTokens:     1_000_000,
		CompletionTokens: 1_000_000,
		TotalTokens:      2_000_000,
	}}, catalog)

	if !got.Rows[0].Fallback {
		t.Fatal("expected fallback row")
	}
	if got.Rows[0].TotalUSD != 10 {
		t.Fatalf("total usd = %f, want 10", got.Rows[0].TotalUSD)
	}
	if got.FallbackCount != 1 {
		t.Fatalf("fallback count = %d", got.FallbackCount)
	}
}
