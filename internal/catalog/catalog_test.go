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
	catalog, err := Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"gpt-4o":{"provider":"openai","input_per_m":2.5,"output_per_m":10}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("gpt-4o")
	if got.Fallback {
		t.Fatal("expected exact pricing, got fallback")
	}
	if got.Pricing.InputPerM != 2.5 || got.Pricing.OutputPerM != 10 {
		t.Fatalf("pricing = %#v", got.Pricing)
	}
}

func TestLookupNormalizesProviderPrefix(t *testing.T) {
	catalog, err := Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"claude-sonnet-4":{"provider":"anthropic","input_per_m":3,"output_per_m":15}}
	}`))
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

func TestLookupFallback(t *testing.T) {
	catalog, err := Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"known":{"provider":"x","input_per_m":1,"output_per_m":2}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Lookup("unknown")
	if !got.Fallback {
		t.Fatal("expected fallback pricing")
	}
	if got.Pricing.InputPerM != 5 || got.Pricing.OutputPerM != 5 {
		t.Fatalf("pricing = %#v", got.Pricing)
	}
}

func TestLoadRejectsInvalidPricing(t *testing.T) {
	_, err := Load([]byte(`{
		"currency":"USD",
		"fallback_per_m":5,
		"models":{"bad":{"provider":"x","input_per_m":-1,"output_per_m":2}}
	}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
}
