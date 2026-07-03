package api

import (
	"encoding/json"
	"net/http"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func (h *Handler) handleCost(w http.ResponseWriter, r *http.Request) {
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
	cat, err := catalog.LoadDefault()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total := costcalc.Calculate(rows, cat)
	json.NewEncoder(w).Encode(total)
}
