package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

var serverStartTime = time.Now()

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonHeader(w)

	// Check store reachability
	if h.db == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	// COUNT(*) is substantially cheaper than building unfiltered model stats and
	// also serves as a reachability check for the database.
	requestsTotal, err := h.db.RequestCount(r.Context())
	if err != nil {
		writeUnavailableError(w, err)
		return
	}

	// DB file size (best effort)
	dbSize := int64(0)
	if fi, err := os.Stat(h.db.DBPath()); err == nil {
		dbSize = fi.Size()
	}
	retention := h.db.RetentionStatus()
	var lastPruneAt any
	if !retention.LastPruneAt.IsZero() {
		lastPruneAt = retention.LastPruneAt.Format(time.RFC3339Nano)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"uptime_seconds": int64(time.Since(serverStartTime).Seconds()),
		"requests_total": requestsTotal,
		"db_size_bytes":  dbSize,
		"retention_days": retention.RetentionDays,
		"last_prune_at":  lastPruneAt,
		"pruned_count":   retention.PrunedCount,
	})
}
