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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		switch p.Mode {
		case policy.AllowAll, policy.Allowlist, policy.Blocklist:
			// valid
		default:
			http.Error(w, "invalid mode: must be allow_all, allowlist, or blocklist", http.StatusBadRequest)
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
				http.Error(w, "models must not contain empty strings", http.StatusBadRequest)
				return
			}
			if seen[m] {
				http.Error(w, fmt.Sprintf("duplicate model pattern: %s", m), http.StatusBadRequest)
				return
			}
			seen[m] = true
		}

		if err := h.db.SetPolicy(r.Context(), &p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonHeader(w)
		json.NewEncoder(w).Encode(p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePolicyModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	models, err := h.db.DistinctModels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonHeader(w)
	json.NewEncoder(w).Encode(models)
}
