package proxy

import (
	"math"
	"testing"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
)

func TestParseRequestMetadata(t *testing.T) {
	meta := ParseRequestMetadata([]byte(`{"messages":[{"role":"user","content":"hi"}],"model":"gpt-4o","stream":true}`))
	if meta.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", meta.Model)
	}
	if !meta.HasStream || !meta.Stream {
		t.Fatalf("stream = %t, hasStream = %t", meta.Stream, meta.HasStream)
	}
}

func TestParseRequestMetadataInvalidJSONReturnsEmptyMetadata(t *testing.T) {
	meta := ParseRequestMetadata([]byte(`not json`))
	if meta.Model != "" {
		t.Fatalf("model = %q, want empty", meta.Model)
	}
	if meta.HasStream || meta.Stream {
		t.Fatalf("stream = %t, hasStream = %t, want empty", meta.Stream, meta.HasStream)
	}
}

func TestFindUsage(t *testing.T) {
	value := map[string]any{
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens":     float64(10),
			"completion_tokens": float64(4),
			"total_tokens":      float64(14),
			"prompt_tokens_details": map[string]any{
				"cached_tokens": float64(6),
			},
		},
	}
	usage, ok := findUsage(value)
	if !ok {
		t.Fatal("usage not found")
	}
	if usage.PromptTokens != 10 || usage.CachedInputTokens != 6 || usage.CompletionTokens != 4 || usage.TotalTokens != 14 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestFindAnthropicUsage(t *testing.T) {
	value := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"usage": map[string]any{
				"input_tokens":                float64(12),
				"cache_read_input_tokens":     float64(5),
				"cache_creation_input_tokens": float64(7),
				"output_tokens":               float64(3),
			},
		},
	}
	usage, ok := findUsage(value)
	if !ok {
		t.Fatal("usage not found")
	}
	// Anthropic input_tokens excludes cache reads; prompt tokens include them.
	if usage.PromptTokens != 17 || usage.CachedInputTokens != 5 || usage.CacheWriteTokens != 7 || usage.CompletionTokens != 3 || usage.TotalTokens != 27 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestAnthropicUsagePricesUncachedInputInFull(t *testing.T) {
	usage, ok := parseUsageObject(map[string]any{
		"input_tokens":                float64(12),
		"cache_read_input_tokens":     float64(5),
		"cache_creation_input_tokens": float64(7),
		"output_tokens":               float64(3),
	})
	if !ok {
		t.Fatal("usage not found")
	}
	pricing := catalog.Pricing{InputPerM: 3, CachedInputPerM: 0.3, CacheWritePerM: 3.75, OutputPerM: 15}
	got := costcalc.CostForUsage(usage.PromptTokens, usage.CachedInputTokens, usage.CacheWriteTokens, usage.CompletionTokens, pricing, false)
	// 12 uncached, 5 cache-read, 7 cache-write and 3 output tokens, per million.
	want := (12*3 + 5*0.3 + 7*3.75 + 3*15) / 1_000_000
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("cost = %.8f, want %.8f", got, want)
	}
}

func TestParseUsageObjectNormalizesPromptTokens(t *testing.T) {
	tests := []struct {
		name  string
		usage map[string]any
		want  Usage
	}{
		{
			name: "anthropic without cache",
			usage: map[string]any{
				"input_tokens":  float64(12),
				"output_tokens": float64(3),
			},
			want: Usage{PromptTokens: 12, CompletionTokens: 3, TotalTokens: 15},
		},
		{
			name: "anthropic fully cached prompt",
			usage: map[string]any{
				"input_tokens":            float64(0),
				"cache_read_input_tokens": float64(5),
				"output_tokens":           float64(3),
			},
			want: Usage{PromptTokens: 5, CachedInputTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		},
		{
			name: "anthropic output-only message_delta",
			usage: map[string]any{
				"output_tokens": float64(3),
			},
			want: Usage{CompletionTokens: 3, TotalTokens: 3},
		},
		{
			name: "responses api input_tokens already include cached tokens",
			usage: map[string]any{
				"input_tokens":  float64(100),
				"output_tokens": float64(20),
				"total_tokens":  float64(120),
			},
			want: Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
		},
		{
			name: "gateway prompt_tokens with anthropic cache read field",
			usage: map[string]any{
				"prompt_tokens":           float64(100),
				"cache_read_input_tokens": float64(80),
				"completion_tokens":       float64(20),
			},
			want: Usage{PromptTokens: 100, CachedInputTokens: 80, CompletionTokens: 20, TotalTokens: 120},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseUsageObject(tt.usage)
			if !ok {
				t.Fatal("usage not found")
			}
			if got != tt.want {
				t.Fatalf("usage = %#v, want %#v", got, tt.want)
			}
		})
	}
}
