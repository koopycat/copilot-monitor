package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"copilot-monitoring/internal/policy"
)

const maxModelDiscoveryResponseBytes = 8 << 20

// isModelDiscoveryResponse reports whether this is a standard OpenAI-compatible
// model-list response that can safely be filtered before it reaches the client.
func isModelDiscoveryResponse(r *http.Request, statusCode int, contentType string, contentEncoding string) bool {
	return r.Method == http.MethodGet &&
		r.URL.Path == "/models" &&
		statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices &&
		strings.Contains(strings.ToLower(contentType), "json") &&
		(contentEncoding == "" || strings.EqualFold(contentEncoding, "identity"))
}

// filterModelDiscoveryResponse filters an OpenAI model-list response according
// to p. The returned reader always contains the original upstream body when it
// cannot be safely transformed, preserving the proxy's fail-open behaviour.
func filterModelDiscoveryResponse(body io.Reader, p *policy.Policy) (io.Reader, bool) {
	if p == nil || (p.Mode != policy.Allowlist && p.Mode != policy.Blocklist) {
		return body, false
	}

	raw, err := io.ReadAll(io.LimitReader(body, maxModelDiscoveryResponseBytes+1))
	if err != nil {
		return io.MultiReader(bytes.NewReader(raw), body), false
	}
	if len(raw) > maxModelDiscoveryResponseBytes {
		return io.MultiReader(bytes.NewReader(raw), body), false
	}

	filtered, ok := filterOpenAIModelList(raw, p)
	if !ok {
		return bytes.NewReader(raw), false
	}
	return bytes.NewReader(filtered), true
}

// filterOpenAIModelList removes entries whose id is disallowed. It preserves
// all top-level and per-model JSON fields other than the data array membership.
func filterOpenAIModelList(body []byte, p *policy.Policy) ([]byte, bool) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false
	}

	rawModels, ok := envelope["data"]
	if !ok {
		return nil, false
	}
	var models []json.RawMessage
	if err := json.Unmarshal(rawModels, &models); err != nil || models == nil {
		return nil, false
	}

	allowed := make([]json.RawMessage, 0, len(models))
	for _, rawModel := range models {
		var model struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawModel, &model); err != nil || model.ID == "" {
			return nil, false
		}
		if p.Allowed(model.ID) {
			allowed = append(allowed, rawModel)
		}
	}

	encodedModels, err := json.Marshal(allowed)
	if err != nil {
		return nil, false
	}
	envelope["data"] = encodedModels
	filtered, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return filtered, true
}
