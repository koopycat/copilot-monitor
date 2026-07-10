//go:build testonly

package proxy

import (
	"net/http"
	"time"
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
