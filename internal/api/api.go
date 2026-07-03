package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"copilot-monitoring/internal/dashboard"
	"copilot-monitoring/internal/store"
)

type Handler struct {
	db        *store.Store
	dashboard http.Handler
}

func NewHandler(db *store.Store) *Handler {
	return &Handler{
		db:        db,
		dashboard: dashboard.DashboardHandler(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/health":
		h.handleHealth(w, r)
	case "/api/stats":
		h.handleStats(w, r)
	case "/api/cost":
		h.handleCost(w, r)
	case "/api/today":
		h.handleToday(w, r)
	case "/api/sessions":
		h.handleSessions(w, r)
	case "/api/session/current":
		h.handleCurrentSession(w, r)
	case "/api/stats/timeline":
		h.handleTimeline(w, r)
	case "/api/export":
		h.handleExport(w, r)
	case "/api/compare":
		h.handleCompare(w, r)
	default:
		h.dashboard.ServeHTTP(w, r)
	}
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func parseSinceParam(r *http.Request) time.Time {
	raw := r.URL.Query().Get("since")
	if raw == "" || raw == "all" {
		return time.Time{}
	}
	if strings.HasSuffix(raw, "d") {
		daysText := strings.TrimSuffix(raw, "d")
		if days, err := strconv.Atoi(daysText); err == nil && days >= 0 {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return time.Now().Add(-d)
	}
	return time.Time{}
}
