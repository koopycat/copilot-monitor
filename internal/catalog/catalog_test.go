package catalog

import "testing"

func TestLoadDefault(t *testing.T) {
	catalog, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", catalog.Currency)
	}
	if len(catalog.Models) == 0 {
		t.Fatal("models were empty")
	}
}

func TestLookupExact(t *testing.T) {
	catalog, err := Load([]byte(testCatalogJSON()))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("gpt-5-mini")
	if got.Fallback {
		t.Fatal("expected exact pricing, got fallback")
	}
	if got.Pricing.InputPerM != 0.25 || got.Pricing.CachedInputPerM != 0.025 || got.Pricing.OutputPerM != 2 {
		t.Fatalf("pricing = %#v", got.Pricing)
	}
}

func TestLookupNormalizesProviderPrefix(t *testing.T) {
	catalog, err := Load([]byte(testCatalogJSON()))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("anthropic/claude-sonnet-4")
	if got.Fallback {
		t.Fatal("expected normalized pricing, got fallback")
	}
	if got.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q", got.Model)
	}
}

func TestLookupProviderFallback(t *testing.T) {
	catalog, err := Load([]byte(testCatalogJSON()))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("claude-new-model")
	if !got.Fallback {
		t.Fatal("expected fallback pricing")
	}
	// Without inferProvider, unknown models use the global fallback
	if got.Pricing.Provider != "unknown" {
		t.Fatalf("provider = %q, want unknown", got.Pricing.Provider)
	}
}

func TestLookupGenericFallback(t *testing.T) {
	catalog, err := Load([]byte(testCatalogJSON()))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("unknown")
	if !got.Fallback {
		t.Fatal("expected fallback pricing")
	}
	if got.Pricing.Provider != "unknown" {
		t.Fatalf("provider = %q", got.Pricing.Provider)
	}
}

func TestLoadRejectsInvalidPricing(t *testing.T) {
	_, err := Load([]byte(`{
		"currency":"USD",
		"fallback":{"provider":"unknown","input_per_m":1,"cached_input_per_m":0.1,"cache_write_per_m":1,"output_per_m":1},
		"provider_fallbacks":{"openai":{"provider":"openai","input_per_m":1,"cached_input_per_m":0.1,"cache_write_per_m":1,"output_per_m":1}},
		"models":{"bad":{"provider":"x","input_per_m":-1,"cached_input_per_m":0,"cache_write_per_m":0,"output_per_m":2}}
	}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLookupNewCatalogModels(t *testing.T) {
	catalog, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		model    string
		expected string
		fallback bool
	}{
		{model: "deepseek-chat", expected: "deepseek", fallback: false},
		{model: "deepseek-reasoner", expected: "deepseek", fallback: false},
		{model: "gpt-4o", expected: "openai", fallback: false},
		{model: "gpt-4o-mini", expected: "openai", fallback: false},
		{model: "gpt-4.1", expected: "openai", fallback: false},
		{model: "claude-3.5-sonnet", expected: "anthropic", fallback: false},
		{model: "claude-3.5-haiku", expected: "anthropic", fallback: false},
		{model: "gemini-2.0-flash", expected: "google", fallback: false},
		{model: "gemini-2.5-flash", expected: "google", fallback: false},
		{model: "o4-mini", expected: "openai", fallback: false},
		{model: "deepseek/deepseek-v4-flash:discounted", expected: "deepseek", fallback: false},
		{model: "deepseek/deepseek-v4-pro:discounted", expected: "deepseek", fallback: false},
		{model: "deepseek-v3", expected: "unknown", fallback: true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := catalog.Lookup(tt.model)
			if got.Fallback != tt.fallback {
				t.Fatalf("fallback = %t, want %t", got.Fallback, tt.fallback)
			}
			if got.Pricing.Provider != tt.expected {
				t.Fatalf("provider = %q, want %q", got.Pricing.Provider, tt.expected)
			}
		})
	}
}

func testCatalogJSON() string {
	return `{
		"currency":"USD",
		"fallback":{"provider":"unknown","input_per_m":5,"cached_input_per_m":0.5,"cache_write_per_m":5,"output_per_m":15},
		"provider_fallbacks":{
			"openai":{"provider":"openai-fallback","input_per_m":2.5,"cached_input_per_m":0.25,"cache_write_per_m":2.5,"output_per_m":15},
			"anthropic":{"provider":"anthropic-fallback","input_per_m":3,"cached_input_per_m":0.3,"cache_write_per_m":3.75,"output_per_m":15}
		},
		"models":{
			"gpt-5-mini":{"provider":"openai","input_per_m":0.25,"cached_input_per_m":0.025,"cache_write_per_m":0.25,"output_per_m":2},
			"claude-sonnet-4":{"provider":"anthropic","input_per_m":3,"cached_input_per_m":0.3,"cache_write_per_m":3.75,"output_per_m":15}
		}
	}`
}
