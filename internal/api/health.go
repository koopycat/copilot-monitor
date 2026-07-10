package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"copilot-monitoring/internal/store"
)

var serverStartTime = time.Now()

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)

	// Check store reachability
	if h.db == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "store is not configured",
		})
		return
	}

	// Verify store is reachable
	if _, err := h.db.DistinctModels(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "store unreachable: " + err.Error(),
		})
		return
	}

	// Total requests from stats
	stats, err := h.db.Stats(r.Context(), store.StatsFilter{})
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "store unreachable: " + err.Error(),
		})
		return
	}

	requestsTotal := int64(0)
	for _, s := range stats {
		requestsTotal += int64(s.Requests)
	}

	// DB file size (best effort)
	dbSize := int64(0)
	if fi, err := os.Stat(store.DefaultPath()); err == nil {
		dbSize = fi.Size()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"uptime_seconds": int64(time.Since(serverStartTime).Seconds()),
		"requests_total": requestsTotal,
		"db_size_bytes":  dbSize,
	})
}
