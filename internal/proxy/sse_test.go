package proxy

import "testing"

func TestSSEObserverDetectsUsageAndModelAcrossSplitLines(t *testing.T) {
	observer := NewSSEObserver()
	observer.Observe([]byte("data: {\"mod"))
	observer.Observe([]byte("el\":\"gpt-4o\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n"))
	observer.Observe([]byte("data: [DONE]\n\n"))
	observer.Finish()

	if !observer.UsageSeen {
		t.Fatal("usage was not detected")
	}
	if observer.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", observer.Model)
	}
	if observer.Usage.PromptTokens != 3 || observer.Usage.CompletionTokens != 2 || observer.Usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", observer.Usage)
	}
	if observer.Bytes == 0 {
		t.Fatal("bytes were not counted")
	}
}

func TestSSEObserverToleratesMalformedJSON(t *testing.T) {
	observer := NewSSEObserver()
	observer.Observe([]byte("data: {broken json}\n"))
	observer.Observe([]byte("data: {\"usage\":{\"total_tokens\":1}}\n"))

	if !observer.UsageSeen {
		t.Fatal("usage was not detected after malformed event")
	}
	if observer.ParseErrors == 0 {
		t.Fatal("expected parse error count")
	}
}

func TestJSONObserverDetectsUsageAndModelAcrossMultilineJSON(t *testing.T) {
	observer := NewJSONObserver()
	observer.Observe([]byte("{\n  \"model\": \"gpt-4o\",\n"))
	observer.Observe([]byte("  \"usage\": {\"prompt_tokens\": 3, \"completion_tokens\": 2, \"total_tokens\": 5}\n}"))
	observer.Finish()

	if !observer.UsageSeen {
		t.Fatal("usage was not detected")
	}
	if observer.Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", observer.Model)
	}
	if observer.Usage.PromptTokens != 3 || observer.Usage.CompletionTokens != 2 || observer.Usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", observer.Usage)
	}
}

func TestSSEObserverMergesUsageAcrossEvents(t *testing.T) {
	const messageStart = `event: message_start
data: {"type":"message_start","message":{"model":"claude-sonnet-4-5","usage":{"input_tokens":12,"cache_creation_input_tokens":7,"cache_read_input_tokens":5,"output_tokens":1}}}

`
	tests := []struct {
		name   string
		stream string
		want   Usage
	}{
		{
			name: "anthropic delta with output tokens only",
			stream: messageStart + `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}

`,
			want: Usage{PromptTokens: 17, CachedInputTokens: 5, CacheWriteTokens: 7, CompletionTokens: 3, TotalTokens: 27},
		},
		{
			name: "anthropic delta with cumulative input and cache tokens",
			stream: messageStart + `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":12,"cache_creation_input_tokens":7,"cache_read_input_tokens":5,"output_tokens":3}}

`,
			want: Usage{PromptTokens: 17, CachedInputTokens: 5, CacheWriteTokens: 7, CompletionTokens: 3, TotalTokens: 27},
		},
		{
			name: "anthropic delta with null input and cache tokens",
			stream: messageStart + `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":null,"cache_creation_input_tokens":null,"cache_read_input_tokens":null,"output_tokens":3}}

`,
			want: Usage{PromptTokens: 17, CachedInputTokens: 5, CacheWriteTokens: 7, CompletionTokens: 3, TotalTokens: 27},
		},
		{
			name: "openai final chunk keeps the reported total",
			stream: `data: {"choices":[{"delta":{"content":"hi"}}]}

data: {"choices":[],"usage":{"prompt_tokens":7,"prompt_tokens_details":{"cached_tokens":2},"cache_write_tokens":2,"completion_tokens":3,"total_tokens":10}}

data: [DONE]

`,
			want: Usage{PromptTokens: 7, CachedInputTokens: 2, CacheWriteTokens: 2, CompletionTokens: 3, TotalTokens: 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer := NewSSEObserver()
			observer.Observe([]byte(tt.stream))
			observer.Finish()

			if !observer.UsageSeen {
				t.Fatal("usage was not detected")
			}
			if observer.Usage != tt.want {
				t.Fatalf("usage = %#v, want %#v", observer.Usage, tt.want)
			}
		})
	}
}
