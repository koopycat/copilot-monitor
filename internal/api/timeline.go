package api

import (
	"encoding/json"
	"net/http"

	"copilot-monitoring/internal/store"
)

func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	granularity := r.URL.Query().Get("granularity")
	if granularity != "hour" {
		granularity = "day"
	}
	filter := store.StatsFilter{
		Since:        parseSinceParam(r),
		Until:        parseUntilParam(r),
		Project:      r.URL.Query().Get("project"),
		Endpoint:     r.URL.Query().Get("endpoint"),
		UpstreamHost: parseUpstreamParam(r),
	}
	buckets, err := h.db.Timeline(r.Context(), filter, granularity)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	json.NewEncoder(w).Encode(buckets)
}
