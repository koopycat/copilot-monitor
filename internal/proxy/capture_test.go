package proxy

import "testing"

func TestParseRequestMetadata(t *testing.T) {
	meta := ParseRequestMetadata([]byte(`{"messages":[{"role":"user","content":"hi"}],"model":"gpt-4o","stream":true}`))
	if meta.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", meta.Model)
	}
	if !meta.HasStream || !meta.Stream {
		t.Fatalf("stream = %t, hasStream = %t", meta.Stream, meta.HasStream)
	}
	if meta.RequestHash == "" {
		t.Fatal("request hash was empty")
	}
}

func TestParseRequestMetadataInvalidJSONStillHashes(t *testing.T) {
	meta := ParseRequestMetadata([]byte(`not json`))
	if meta.Model != "" {
		t.Fatalf("model = %q, want empty", meta.Model)
	}
	if meta.RequestHash == "" {
		t.Fatal("request hash was empty")
	}
}

func TestFindUsage(t *testing.T) {
	value := map[string]any{
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens":     float64(10),
			"completion_tokens": float64(4),
			"total_tokens":      float64(14),
		},
	}
	usage, ok := findUsage(value)
	if !ok {
		t.Fatal("usage not found")
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 4 || usage.TotalTokens != 14 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestFindAnthropicUsage(t *testing.T) {
	value := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"usage": map[string]any{
				"input_tokens":  float64(12),
				"output_tokens": float64(3),
			},
		},
	}
	usage, ok := findUsage(value)
	if !ok {
		t.Fatal("usage not found")
	}
	if usage.PromptTokens != 12 || usage.CompletionTokens != 3 || usage.TotalTokens != 15 {
		t.Fatalf("usage = %#v", usage)
	}
}
