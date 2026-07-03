package api

import (
	"encoding/json"
	"net/http"
	"time"

	"copilot-monitoring/internal/store"
)

func (h *Handler) handleToday(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	filter := store.StatsFilter{
		Since:    start,
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
