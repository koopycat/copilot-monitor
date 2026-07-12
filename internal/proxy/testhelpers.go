//go:build testonly

package proxy

import (
	"net/http"
	"time"

	"copilot-monitoring/internal/compression/headroom"
)

// ExpirePolicyCache sets the policy cache TTL to the zero time so the next
// request will re-fetch the policy from the store.
func (h *Handler) ExpirePolicyCache() {
	h.policyUntil = time.Time{}
}

// SetTestClient replaces the internal HTTP client. Used in tests that need
// to control transport (e.g., TLS skip or mock round trips).
func (h *Handler) SetTestClient(c *http.Client) {
	h.client = c
}

// SetCompressor injects a compressor into the handler's client cache for the
// given endpoint. This allows tests to supply a mock or a client pointed at a
// fake Headroom server without requiring a real loopback endpoint.
func (h *Handler) SetCompressor(endpoint string, c headroom.MessageCompressor) {
	h.compressorMu.Lock()
	defer h.compressorMu.Unlock()
	if h.compressorCache == nil {
		h.compressorCache = make(map[string]headroom.MessageCompressor)
	}
	h.compressorCache[endpoint] = c
}
