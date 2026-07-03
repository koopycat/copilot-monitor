package proxy

import (
	"bytes"
	"encoding/json"
)

type RequestMetadata struct {
	Model     string
	Stream    bool
	HasStream bool
}

type Usage struct {
	PromptTokens      int
	CachedInputTokens int
	CacheWriteTokens  int
	CompletionTokens  int
	TotalTokens       int
}

func ParseRequestMetadata(body []byte) RequestMetadata {
	var meta RequestMetadata
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return meta
	}

	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return meta
	}
	if model, ok := findStringKey(value, "model"); ok {
		meta.Model = model
	}
	if stream, ok := findBoolKey(value, "stream"); ok {
		meta.Stream = stream
		meta.HasStream = true
	}
	return meta
}

func findStringKey(value any, key string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed[key]; ok {
			if s, ok := raw.(string); ok {
				return s, true
			}
		}
		for _, v := range typed {
			if s, ok := findStringKey(v, key); ok {
				return s, true
			}
		}
	case []any:
		for _, item := range typed {
			if s, ok := findStringKey(item, key); ok {
				return s, true
			}
		}
	}
	return "", false
}

func findBoolKey(value any, key string) (bool, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed[key]; ok {
			if b, ok := raw.(bool); ok {
				return b, true
			}
		}
		for _, v := range typed {
			if b, ok := findBoolKey(v, key); ok {
				return b, true
			}
		}
	case []any:
		for _, item := range typed {
			if b, ok := findBoolKey(item, key); ok {
				return b, true
			}
		}
	}
	return false, false
}

func findUsage(value any) (Usage, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed["usage"]; ok {
			if usage, ok := parseUsageObject(raw); ok {
				return usage, true
			}
		}
		for _, v := range typed {
			if usage, ok := findUsage(v); ok {
				return usage, true
			}
		}
	case []any:
		for _, item := range typed {
			if usage, ok := findUsage(item); ok {
				return usage, true
			}
		}
	}
	return Usage{}, false
}

func parseUsageObject(value any) (Usage, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return Usage{}, false
	}

	promptTokens := intFromJSONNumber(m["prompt_tokens"])
	completionTokens := intFromJSONNumber(m["completion_tokens"])
	cachedInputTokens := intFromJSONNumber(m["cached_input_tokens"])
	cacheWriteTokens := intFromJSONNumber(m["cache_write_tokens"])
	anthropicStyle := false

	if details, ok := m["prompt_tokens_details"].(map[string]any); ok && cachedInputTokens == 0 {
		cachedInputTokens = intFromJSONNumber(details["cached_tokens"])
	}
	if promptTokens == 0 {
		if inputTokens := intFromJSONNumber(m["input_tokens"]); inputTokens != 0 {
			promptTokens = inputTokens
			anthropicStyle = true
		}
	}
	if completionTokens == 0 {
		if outputTokens := intFromJSONNumber(m["output_tokens"]); outputTokens != 0 {
			completionTokens = outputTokens
			anthropicStyle = true
		}
	}
	if cachedInputTokens == 0 {
		if cacheReadTokens := intFromJSONNumber(m["cache_read_input_tokens"]); cacheReadTokens != 0 {
			cachedInputTokens = cacheReadTokens
			anthropicStyle = true
		}
	}
	if cacheWriteTokens == 0 {
		if cacheCreationTokens := intFromJSONNumber(m["cache_creation_input_tokens"]); cacheCreationTokens != 0 {
			cacheWriteTokens = cacheCreationTokens
			anthropicStyle = true
		}
	}

	usage := Usage{
		PromptTokens:      promptTokens,
		CachedInputTokens: cachedInputTokens,
		CacheWriteTokens:  cacheWriteTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       intFromJSONNumber(m["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		if anthropicStyle {
			usage.TotalTokens += usage.CachedInputTokens + usage.CacheWriteTokens
		} else {
			usage.TotalTokens += usage.CacheWriteTokens
		}
	}
	return usage, usage.PromptTokens != 0 || usage.CachedInputTokens != 0 || usage.CacheWriteTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0
}

func intFromJSONNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		return 0
	}
}
