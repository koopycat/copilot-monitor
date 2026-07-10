package api

import (
	"encoding/json"
	"net/http"

	"llm-proxy/internal/catalog"
	costcalc "llm-proxy/internal/cost"
	"llm-proxy/internal/store"
)

func (h *Handler) handleCost(w http.ResponseWriter, r *http.Request) {
	filter := store.StatsFilter{
		Since:        parseSinceParam(r),
		Until:        parseUntilParam(r),
		Project:      r.URL.Query().Get("project"),
		Endpoint:     r.URL.Query().Get("endpoint"),
		UpstreamHost: parseUpstreamParam(r),
	}
	rows, err := h.db.Stats(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total := costcalc.Calculate(rows, cat)
	json.NewEncoder(w).Encode(total)
}
