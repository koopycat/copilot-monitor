package api

import (
	"encoding/json"
	"net/http"

	"copilot-monitoring/internal/store"
)

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	filter := store.StatsFilter{
		Since:        parseSinceParam(r),
		Until:        parseUntilParam(r),
		Project:      r.URL.Query().Get("project"),
		Endpoint:     r.URL.Query().Get("endpoint"),
		UpstreamHost: parseUpstreamParam(r),
	}
	rows, err := h.db.Stats(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	json.NewEncoder(w).Encode(rows)
}
