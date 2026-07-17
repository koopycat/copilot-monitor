package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/store"
)

type Handler struct {
	db *store.Store

	catalogOnce sync.Once
	catalog     catalog.Catalog
	catalogErr  error
}

func NewHandler(db *store.Store) *Handler {
	return &Handler{
		db: db,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
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
	case "/api/sessions/count":
		h.handleSessionsCount(w, r)
	case "/api/sessions/distinct-projects":
		h.handleDistinctProjects(w, r)
	case "/api/session/current":
		h.handleCurrentSession(w, r)
	case "/api/stats/timeline":
		h.handleTimeline(w, r)
	case "/api/export":
		h.handleExport(w, r)
	case "/api/upstreams":
		h.handleUpstreams(w, r)
	case "/api/policy":
		h.handlePolicy(w, r)
	case "/api/policy/models":
		h.handlePolicyModels(w, r)
	case "/api/anomalies":
		h.handleAnomalies(w, r)
	default:
		writeJSONError(w, http.StatusNotFound, "not found")
	}
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	jsonHeader(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("api request failed: %v", err)
	writeJSONError(w, http.StatusInternalServerError, "internal server error")
}

func writeUnavailableError(w http.ResponseWriter, err error) {
	log.Printf("api health check failed: %v", err)
	writeJSONError(w, http.StatusServiceUnavailable, "service unavailable")
}

// catalogDefault keeps the pricing catalog in the handler rather than loading
// and parsing the embedded file for each cost-bearing API request.
func (h *Handler) catalogDefault() (catalog.Catalog, error) {
	h.catalogOnce.Do(func() {
		h.catalog, h.catalogErr = h.db.Catalog()
	})
	return h.catalog, h.catalogErr
}

func parseTimeParam(r *http.Request, key string) time.Time {
	raw := r.URL.Query().Get(key)
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
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", raw); err == nil {
		return t
	}
	return time.Time{}
}

func parseSinceParam(r *http.Request) time.Time {
	return parseTimeParam(r, "since")
}

func parseUntilParam(r *http.Request) time.Time {
	return parseTimeParam(r, "until")
}

func parseUpstreamParam(r *http.Request) string {
	return r.URL.Query().Get("upstream")
}

func (h *Handler) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	hosts, err := h.db.DistinctUpstreamHosts(r.Context())
	if err != nil {
		writeInternalError(w, err)
		return
	}
	json.NewEncoder(w).Encode(hosts)
}
