package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed models.json
var catalogFS embed.FS

type Catalog struct {
	Currency          string             `json:"currency"`
	Fallback          Pricing            `json:"fallback"`
	ProviderFallbacks map[string]Pricing `json:"provider_fallbacks"`
	Models            map[string]Pricing `json:"models"`
}

type Pricing struct {
	Provider        string  `json:"provider"`
	InputPerM       float64 `json:"input_per_m"`
	CachedInputPerM float64 `json:"cached_input_per_m"`
	CacheWritePerM  float64 `json:"cache_write_per_m"`
	OutputPerM      float64 `json:"output_per_m"`
}

type LookupResult struct {
	Model    string
	Pricing  Pricing
	Fallback bool
}

func LoadDefault() (Catalog, error) {
	data, err := catalogFS.ReadFile("models.json")
	if err != nil {
		return Catalog{}, err
	}
	return Load(data)
}

func Load(data []byte) (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func (c Catalog) Validate() error {
	if c.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if err := validatePricing("fallback", c.Fallback); err != nil {
		return err
	}
	if len(c.ProviderFallbacks) == 0 {
		return fmt.Errorf("at least one provider fallback is required")
	}
	for provider, pricing := range c.ProviderFallbacks {
		if strings.TrimSpace(provider) == "" {
			return fmt.Errorf("provider fallback key must not be empty")
		}
		if err := validatePricing("provider fallback "+provider, pricing); err != nil {
			return err
		}
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("at least one model price is required")
	}
	for model, pricing := range c.Models {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name must not be empty")
		}
		if err := validatePricing("model "+model, pricing); err != nil {
			return err
		}
	}
	return nil
}

func validatePricing(name string, pricing Pricing) error {
	if pricing.Provider == "" {
		return fmt.Errorf("provider is required for %s", name)
	}
	if pricing.InputPerM < 0 || pricing.CachedInputPerM < 0 || pricing.CacheWritePerM < 0 || pricing.OutputPerM < 0 {
		return fmt.Errorf("prices must be non-negative for %s", name)
	}
	return nil
}

func (c Catalog) Lookup(model string) LookupResult {
	normalized := normalizeModel(model)
	if pricing, ok := c.Models[normalized]; ok {
		return LookupResult{Model: normalized, Pricing: pricing}
	}
	provider := inferProvider(normalized)
	if provider != "" {
		if pricing, ok := c.ProviderFallbacks[provider]; ok {
			return LookupResult{Model: normalized, Pricing: pricing, Fallback: true}
		}
	}
	return LookupResult{Model: normalized, Pricing: c.Fallback, Fallback: true}
}

func normalizeModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "openai/")
	model = strings.TrimPrefix(model, "anthropic/")
	model = strings.TrimPrefix(model, "google/")
	model = strings.TrimPrefix(model, "github/")
	model = strings.TrimPrefix(model, "microsoft/")
	model = strings.ReplaceAll(model, " ", "-")
	model = strings.ReplaceAll(model, "_", "-")
	return model
}

func inferProvider(model string) string {
	switch {
	case strings.Contains(model, "claude"):
		return "anthropic"
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4"):
		return "openai"
	case strings.Contains(model, "gemini"):
		return "google"
	case strings.Contains(model, "raptor"):
		return "github"
	case strings.Contains(model, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(model, "mai-"):
		return "microsoft"
	default:
		return ""
	}
}
