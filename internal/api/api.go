package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"copilot-monitoring/internal/catalog"
	costcalc "copilot-monitoring/internal/cost"
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
	case "/api/stats/timeline":
		h.handleTimeline(w, r)
	case "/api/export":
		h.handleExport(w, r)
	default:
		h.dashboard.ServeHTTP(w, r)
	}
}

func jsonHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
	})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	since := parseSinceParam(r)
	filter := store.StatsFilter{
		Since:    since,
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

func (h *Handler) handleCost(w http.ResponseWriter, r *http.Request) {
	since := parseSinceParam(r)
	filter := store.StatsFilter{
		Since:    since,
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

func (h *Handler) handleToday(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	filter := store.StatsFilter{
		Since:    start,
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

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	since := parseSinceParam(r)
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
		Since:   since,
		Project: r.URL.Query().Get("project"),
		Limit:   limit,
	}
	rows, err := h.db.Sessions(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(rows)
}

func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)
	since := parseSinceParam(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity != "hour" {
		granularity = "day"
	}
	filter := store.StatsFilter{
		Since:    since,
		Project:  r.URL.Query().Get("project"),
		Endpoint: r.URL.Query().Get("endpoint"),
	}
	buckets, err := h.db.Timeline(r.Context(), filter, granularity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(buckets)
}

func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	since := parseSinceParam(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	rows, err := h.db.ExportRequests(r.Context(), since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=copilot-export.csv")
		w.Write([]byte("ts,endpoint,model,status,latency_ms,prompt_tokens,cached_input_tokens,cache_write_tokens,completion_tokens,total_tokens,project\n"))
		for _, row := range rows {
			fmt.Fprintf(w, "%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%s\n",
				row.Timestamp, row.Endpoint, csvEscape(row.Model), row.Status, row.LatencyMS,
				row.PromptTokens, row.CachedInputTokens, row.CacheWriteTokens,
				row.CompletionTokens, row.TotalTokens, csvEscape(row.Project))
		}
	default:
		jsonHeader(w)
		json.NewEncoder(w).Encode(rows)
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
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
