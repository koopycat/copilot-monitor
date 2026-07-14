package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestSessionsCursorCountAndDistinctProjects(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i, project := range []string{"alpha", "bravo", "charlie"} {
		if err := st.InsertRequest(ctx, store.RequestRecord{
			Timestamp: base.Add(time.Duration(i) * time.Hour), Endpoint: "chat", Method: "POST", Path: "/chat",
			UpstreamHost: "api.example.test", Model: "gpt-4o", Status: 200, Project: project,
		}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(st)

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/sessions?limit=2", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first page status = %d: %s", first.Code, first.Body.String())
	}
	var page []store.SessionStats
	if err := json.NewDecoder(first.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Project != "charlie" || page[1].Project != "bravo" {
		t.Fatalf("first page = %#v", page)
	}

	second := httptest.NewRecorder()
	cursor := page[1].StartedAt.Format(time.RFC3339Nano)
	url := "/api/sessions?limit=2&cursor=" + cursor + "&cursor_id=" + strconv.FormatInt(page[1].ID, 10)
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, url, nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second page status = %d: %s", second.Code, second.Body.String())
	}
	var remaining []store.SessionStats
	if err := json.NewDecoder(second.Body).Decode(&remaining); err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Project != "alpha" {
		t.Fatalf("second page = %#v", remaining)
	}

	count := httptest.NewRecorder()
	h.ServeHTTP(count, httptest.NewRequest(http.MethodGet, "/api/sessions/count", nil))
	var countResponse map[string]int
	if err := json.NewDecoder(count.Body).Decode(&countResponse); err != nil {
		t.Fatal(err)
	}
	if countResponse["count"] != 3 {
		t.Fatalf("count = %#v", countResponse)
	}
	filteredCount := httptest.NewRecorder()
	h.ServeHTTP(filteredCount, httptest.NewRequest(http.MethodGet, "/api/sessions/count?project=bravo", nil))
	if err := json.NewDecoder(filteredCount.Body).Decode(&countResponse); err != nil {
		t.Fatal(err)
	}
	if countResponse["count"] != 1 {
		t.Fatalf("filtered count = %#v", countResponse)
	}

	projects := httptest.NewRecorder()
	h.ServeHTTP(projects, httptest.NewRequest(http.MethodGet, "/api/sessions/distinct-projects", nil))
	var names []string
	if err := json.NewDecoder(projects.Body).Decode(&names); err != nil {
		t.Fatal(err)
	}
	if got := len(names); got != 3 || names[0] != "alpha" || names[2] != "charlie" {
		t.Fatalf("projects = %#v", names)
	}
}

func TestAnomaliesEndpointAndSanitizedErrors(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 51; i++ {
		category, severity := "parse_error", "warn"
		if i == 50 {
			category, severity = "auth_missing", "error"
		}
		if err := st.WriteAnomaly(ctx, store.AnomalyRecord{Timestamp: now.Add(time.Duration(i) * time.Second), Category: category, Severity: severity}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(st)

	all := httptest.NewRecorder()
	h.ServeHTTP(all, httptest.NewRequest(http.MethodGet, "/api/anomalies", nil))
	var anomalies []store.Anomaly
	if err := json.NewDecoder(all.Body).Decode(&anomalies); err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 50 || anomalies[0].Category != "auth_missing" {
		t.Fatalf("anomalies = %#v", anomalies)
	}

	filtered := httptest.NewRecorder()
	h.ServeHTTP(filtered, httptest.NewRequest(http.MethodGet, "/api/anomalies?severity=error", nil))
	if err := json.NewDecoder(filtered.Body).Decode(&anomalies); err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 || anomalies[0].Category != "auth_missing" {
		t.Fatalf("filtered anomalies = %#v", anomalies)
	}

	failed := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if failed.Code != http.StatusInternalServerError || failed.Body.String() != "{\"error\":\"internal server error\"}\n" {
		t.Fatalf("sanitized response = %d %q", failed.Code, failed.Body.String())
	}
}
