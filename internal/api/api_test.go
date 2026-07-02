package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestCompareEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, rec := range []store.RequestRecord{
		{Timestamp: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), Endpoint: "chat", Model: "gpt-5-mini", Status: 200, PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500},
		{Timestamp: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), Endpoint: "chat", Model: "gpt-5-mini", Status: 200, PromptTokens: 2000, CompletionTokens: 1000, TotalTokens: 3000},
	} {
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/compare?a=2026-06&b=2026-07", nil)
	rr := httptest.NewRecorder()
	NewHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response compareResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Periods) != 2 {
		t.Fatalf("periods = %d, want 2", len(response.Periods))
	}
	if response.Periods[0].Label != "2026-06" || response.Periods[1].Label != "2026-07" {
		t.Fatalf("response = %#v", response)
	}
	if response.Periods[0].TotalTokens != 1500 || response.Periods[1].TotalTokens != 3000 {
		t.Fatalf("response = %#v", response)
	}
	if response.Periods[1].TotalCost <= response.Periods[0].TotalCost {
		t.Fatalf("expected July cost to be higher: %#v", response)
	}
}

func TestCompareEndpointRejectsPartialMonthParams(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/compare?a=2026-06", nil)
	rr := httptest.NewRecorder()
	NewHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
