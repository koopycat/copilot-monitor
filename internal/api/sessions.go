package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/store"
)

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	filter := store.SessionFilter{
		Since:   parseSinceParam(r),
		Until:   parseUntilParam(r),
		Project: r.URL.Query().Get("project"),
		Limit:   limit,
	}
	cursor, cursorID := r.URL.Query().Get("cursor"), r.URL.Query().Get("cursor_id")
	if cursor != "" || cursorID != "" {
		if cursor == "" || cursorID == "" {
			writeJSONError(w, http.StatusBadRequest, "cursor and cursor_id must be provided together")
			return
		}
		startedAt, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		id, err := strconv.ParseInt(cursorID, 10, 64)
		if err != nil || id <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor_id")
			return
		}
		filter.CursorStartedAt = startedAt
		filter.CursorID = id
	}
	rows, err := h.db.Sessions(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	ids := make([]int64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	modelsBySession, err := h.db.SessionModelsBatch(r.Context(), ids)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	cat, err := h.catalogDefault()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	for i := range rows {
		rows[i].Cost = costcalc.Calculate(modelsBySession[rows[i].ID], cat).TotalUSD
	}
	_ = json.NewEncoder(w).Encode(rows)
}

func (h *Handler) handleSessionsCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.db.CountSessions(r.Context(), store.SessionFilter{
		Since:   parseSinceParam(r),
		Until:   parseUntilParam(r),
		Project: r.URL.Query().Get("project"),
	})
	if err != nil {
		writeInternalError(w, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func (h *Handler) handleDistinctProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.db.DistinctProjects(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	_ = json.NewEncoder(w).Encode(projects)
}

func (h *Handler) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := h.db.QueryAnomalies(r.Context(), store.AnomalyFilter{
		Category: r.URL.Query().Get("category"),
		Severity: r.URL.Query().Get("severity"),
		Limit:    50,
	})
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if anomalies == nil {
		anomalies = []store.Anomaly{}
	}
	_ = json.NewEncoder(w).Encode(anomalies)
}
