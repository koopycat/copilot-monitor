package proxy

import (
	"encoding/json"
	"net/http"
	"os"

	"llm-proxy/internal/store"
)

// HealthResponse is the JSON response for /_health.
type HealthResponse struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	RequestsTotal int64  `json:"requests_total"`
	DBSizeBytes   int64  `json:"db_size_bytes"`
}

// HandleHealth handles /_health for the proxy-level handler.
// It uses the store and the proxy's request counter and start time.
func HandleHealth(w http.ResponseWriter, r *http.Request, st *store.Store, requestCount int64, uptimeSeconds int64) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// Check store reachability
	if st == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "store is not configured",
		})
		return
	}

	// Try to query the store to verify it's reachable
	if _, err := st.DistinctModels(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "store unreachable: " + err.Error(),
		})
		return
	}

	// Get DB file size (best effort)
	dbSize := int64(0)
	if fi, err := os.Stat(store.DefaultPath()); err == nil {
		dbSize = fi.Size()
	}

	resp := HealthResponse{
		Status:        "ok",
		UptimeSeconds: uptimeSeconds,
		RequestsTotal: requestCount,
		DBSizeBytes:   dbSize,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
