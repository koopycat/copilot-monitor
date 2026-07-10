package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if err := h.db.RebuildSessions(context.Background(), 30*time.Minute); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	filter := store.SessionFilter{
		Since:        parseSinceParam(r),
		Until:        parseUntilParam(r),
		Project:      r.URL.Query().Get("project"),
		UpstreamHost: parseUpstreamParam(r),
		Limit:        limit,
	}
	rows, err := h.db.Sessions(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cat, err := catalog.LoadDefault()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range rows {
		models, err := h.db.SessionModels(r.Context(), rows[i].ID)
		if err != nil {
			continue
		}
		rows[i].Cost = costcalc.Calculate(models, cat).TotalUSD
	}
	json.NewEncoder(w).Encode(rows)
}
