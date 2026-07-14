package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"copilot-monitoring/internal/policy"
)

func (h *Handler) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p, err := h.db.GetPolicy(r.Context())
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if p == nil {
			p = policy.DefaultPolicy()
		}
		if p.Models == nil {
			p.Models = []string{}
		}
		jsonHeader(w)
		json.NewEncoder(w).Encode(p)

	case http.MethodPut:
		var p policy.Policy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		switch p.Mode {
		case policy.AllowAll, policy.Allowlist, policy.Blocklist:
			// valid
		default:
			writeJSONError(w, http.StatusBadRequest, "invalid mode: must be allow_all, allowlist, or blocklist")
			return
		}
		if p.Models == nil {
			p.Models = []string{}
		}

		// Validate and clean models
		seen := make(map[string]bool, len(p.Models))
		for i, m := range p.Models {
			m = strings.TrimSpace(m)
			p.Models[i] = m
			if m == "" {
				writeJSONError(w, http.StatusBadRequest, "models must not contain empty strings")
				return
			}
			if seen[m] {
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("duplicate model pattern: %s", m))
				return
			}
			seen[m] = true
		}

		if err := h.db.SetPolicy(r.Context(), &p); err != nil {
			writeInternalError(w, err)
			return
		}
		jsonHeader(w)
		json.NewEncoder(w).Encode(p)

	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handlePolicyModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	models, err := h.db.DistinctModels(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	jsonHeader(w)
	json.NewEncoder(w).Encode(models)
}
