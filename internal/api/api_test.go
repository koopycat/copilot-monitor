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

func TestCurrentSessionEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, rec := range []store.RequestRecord{
		{Timestamp: time.Now().UTC().Add(-5 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-5-mini", Status: 200, PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500},
		{Timestamp: time.Now().UTC().Add(-4 * time.Minute), Endpoint: "chat", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.githubcopilot.com", Model: "gpt-5-mini", Status: 200, PromptTokens: 2000, CompletionTokens: 1000, TotalTokens: 3000},
	} {
		if err := st.InsertRequest(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/session/current", nil)
	rr := httptest.NewRecorder()
	NewHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response currentSessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Session == nil {
		t.Fatal("session is nil")
	}
	if !response.Session.Active || response.Session.Status != "active" {
		t.Fatalf("session = %#v", response.Session)
	}
	if response.Session.RequestCount != 2 || response.Session.TokenCount != 4500 {
		t.Fatalf("session = %#v", response.Session)
	}
	if response.Session.Cost <= 0 {
		t.Fatalf("cost = %f, want positive", response.Session.Cost)
	}
	if len(response.Models) != 1 || response.Models[0].Model != "gpt-5-mini" || response.Models[0].Requests != 2 {
		t.Fatalf("models = %#v", response.Models)
	}
}

func TestCurrentSessionEndpointEmptyDB(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/session/current", nil)
	rr := httptest.NewRecorder()
	NewHandler(st).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response currentSessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Session != nil {
		t.Fatalf("session = %#v, want nil", response.Session)
	}
	if len(response.Models) != 0 {
		t.Fatalf("models = %#v, want empty", response.Models)
	}
}
