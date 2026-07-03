package api

import (
	"encoding/json"
	"net/http"

	"copilot-monitoring/internal/store"
)

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	filter := store.StatsFilter{
		Since:    parseSinceParam(r),
		Until:    parseUntilParam(r),
		Project:  r.URL.Query().Get("project"),
		Endpoint: r.URL.Query().Get("endpoint"),
	}
	rows, err := h.db.Stats(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(rows)
}
