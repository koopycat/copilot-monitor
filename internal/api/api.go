package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"
)

type Handler struct {
	db           *store.Store
	routesConfig *proxy.ProxyConfig
}

func NewHandler(db *store.Store) *Handler {
	return NewHandlerWithConfig(db, nil)
}

func NewHandlerWithConfig(db *store.Store, cfg *proxy.ProxyConfig) *Handler {
	return &Handler{
		db:           db,
		routesConfig: cfg,
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
	case "/api/upstreams":
		h.handleUpstreams(w, r)
	case "/api/config":
		h.handleConfig(w, r)
	case "/api/policy":
		h.handlePolicy(w, r)
	case "/api/policy/models":
		h.handlePolicyModels(w, r)
	default:
		http.NotFound(w, r)
	}
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(hosts)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	if h.routesConfig == nil {
		json.NewEncoder(w).Encode(map[string][]any{"routes": {}})
		return
	}
	json.NewEncoder(w).Encode(h.routesConfig)
}
