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
	if got.Pricing.Provider != "anthropic-fallback" {
		t.Fatalf("provider = %q", got.Pricing.Provider)
	}
	if got.Pricing.CacheWritePerM != 3.75 {
		t.Fatalf("pricing = %#v", got.Pricing)
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
