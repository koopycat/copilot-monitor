package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type RequestMetadata struct {
	Model       string
	Stream      bool
	HasStream   bool
	RequestHash string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func ParseRequestMetadata(body []byte) RequestMetadata {
	var meta RequestMetadata
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return meta
	}

	hash := sha256.Sum256(trimmed)
	meta.RequestHash = hex.EncodeToString(hash[:])

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

func findRawUsageObjects(value any) []json.RawMessage {
	var out []json.RawMessage
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed["usage"]; ok {
			if encoded, err := json.Marshal(raw); err == nil {
				out = append(out, encoded)
			}
		}
		for _, v := range typed {
			out = append(out, findRawUsageObjects(v)...)
		}
	case []any:
		for _, item := range typed {
			out = append(out, findRawUsageObjects(item)...)
		}
	}
	return out
}

func parseUsageObject(value any) (Usage, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return Usage{}, false
	}
	promptTokens := intFromJSONNumber(m["prompt_tokens"])
	completionTokens := intFromJSONNumber(m["completion_tokens"])
	if promptTokens == 0 {
		promptTokens = intFromJSONNumber(m["input_tokens"])
	}
	if completionTokens == 0 {
		completionTokens = intFromJSONNumber(m["output_tokens"])
	}
	usage := Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      intFromJSONNumber(m["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0
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
