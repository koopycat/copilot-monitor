package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestHealthIncludesRetentionStatus(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	prunedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	st.SetRetentionDays(90)
	st.RecordPrune(prunedAt, 12)

	recorder := httptest.NewRecorder()
	NewHandler(st).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		RetentionDays int    `json:"retention_days"`
		LastPruneAt   string `json:"last_prune_at"`
		PrunedCount   int    `json:"pruned_count"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.RetentionDays != 90 || response.LastPruneAt != prunedAt.Format(time.RFC3339Nano) || response.PrunedCount != 12 {
		t.Fatalf("health retention = %#v", response)
	}
}
