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
	} else if model, ok := findNestedModel(value); ok {
		meta.Model = model
	}
	if stream, ok := findBoolKey(value, "stream"); ok {
		meta.Stream = stream
		meta.HasStream = true
	}
	return meta
}

func findStringKey(value any, key string) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	if raw, ok := m[key]; ok {
		if s, ok := raw.(string); ok {
			return s, true
		}
	}
	return "", false
}

// findNestedModel checks for the common Copilot Responses API nested path response.model.
// It only descends one level into the "response" key — not arbitrary depth.
func findNestedModel(value any) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	respRaw, ok := m["response"]
	if !ok {
		return "", false
	}
	respMap, ok := respRaw.(map[string]any)
	if !ok {
		return "", false
	}
	raw, ok := respMap["model"]
	if !ok {
		return "", false
	}
	s, ok := raw.(string)
	return s, ok
}

func findBoolKey(value any, key string) (bool, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return false, false
	}
	if raw, ok := m[key]; ok {
		if b, ok := raw.(bool); ok {
			return b, true
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

	// Usage is normalized to the OpenAI convention: PromptTokens includes cache
	// reads (CachedInputTokens is a subset of it) and excludes cache writes.
	promptTokens := intFromJSONNumber(m["prompt_tokens"])
	completionTokens := intFromJSONNumber(m["completion_tokens"])
	cachedInputTokens := intFromJSONNumber(m["cached_input_tokens"])
	cacheWriteTokens := intFromJSONNumber(m["cache_write_tokens"])
	promptExcludesCacheReads := promptTokens == 0

	if details, ok := m["prompt_tokens_details"].(map[string]any); ok && cachedInputTokens == 0 {
		cachedInputTokens = intFromJSONNumber(details["cached_tokens"])
	}
	if promptTokens == 0 {
		promptTokens = intFromJSONNumber(m["input_tokens"])
	}
	if completionTokens == 0 {
		completionTokens = intFromJSONNumber(m["output_tokens"])
	}
	if cachedInputTokens == 0 {
		if cacheReadTokens := intFromJSONNumber(m["cache_read_input_tokens"]); cacheReadTokens != 0 {
			cachedInputTokens = cacheReadTokens
			// Anthropic input_tokens counts only the uncached input.
			if promptExcludesCacheReads {
				promptTokens += cacheReadTokens
			}
		}
	}
	if cacheWriteTokens == 0 {
		cacheWriteTokens = intFromJSONNumber(m["cache_creation_input_tokens"])
	}

	usage := Usage{
		PromptTokens:      promptTokens,
		CachedInputTokens: cachedInputTokens,
		CacheWriteTokens:  cacheWriteTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       intFromJSONNumber(m["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens + usage.CacheWriteTokens
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
