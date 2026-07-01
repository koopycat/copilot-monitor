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
	Currency     string             `json:"currency"`
	FallbackPerM float64            `json:"fallback_per_m"`
	Models       map[string]Pricing `json:"models"`
}

type Pricing struct {
	Provider   string  `json:"provider"`
	InputPerM  float64 `json:"input_per_m"`
	OutputPerM float64 `json:"output_per_m"`
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
	if c.FallbackPerM < 0 {
		return fmt.Errorf("fallback_per_m must be non-negative")
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("at least one model price is required")
	}
	for model, pricing := range c.Models {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name must not be empty")
		}
		if pricing.Provider == "" {
			return fmt.Errorf("provider is required for model %q", model)
		}
		if pricing.InputPerM < 0 || pricing.OutputPerM < 0 {
			return fmt.Errorf("prices must be non-negative for model %q", model)
		}
	}
	return nil
}

func (c Catalog) Lookup(model string) LookupResult {
	if pricing, ok := c.Models[model]; ok {
		return LookupResult{Model: model, Pricing: pricing}
	}
	if normalized := normalizeModel(model); normalized != model {
		if pricing, ok := c.Models[normalized]; ok {
			return LookupResult{Model: normalized, Pricing: pricing}
		}
	}
	return LookupResult{
		Model: model,
		Pricing: Pricing{
			Provider:   "fallback",
			InputPerM:  c.FallbackPerM,
			OutputPerM: c.FallbackPerM,
		},
		Fallback: true,
	}
}

func normalizeModel(model string) string {
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "openai/")
	model = strings.TrimPrefix(model, "anthropic/")
	model = strings.TrimPrefix(model, "google/")
	return model
}
