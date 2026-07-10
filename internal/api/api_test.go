package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"copilot-monitoring/internal/policy"
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

func TestGetPolicyDefault(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()
	h := NewHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/policy", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var p policy.Policy
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
	assert.Equal(t, policy.AllowAll, p.Mode)
	assert.Empty(t, p.Models)
}

func TestPutAndGetPolicy(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()
	h := NewHandler(s)

	// PUT valid blocklist
	putBody := bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(putBody).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}}))
	req := httptest.NewRequest(http.MethodPut, "/api/policy", putBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var putResp policy.Policy
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&putResp))
	assert.Equal(t, policy.Blocklist, putResp.Mode)
	assert.Equal(t, []string{"gpt-4o"}, putResp.Models)

	// GET round-trip
	req2 := httptest.NewRequest(http.MethodGet, "/api/policy", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	var getResp policy.Policy
	require.NoError(t, json.NewDecoder(rec2.Body).Decode(&getResp))
	assert.Equal(t, policy.Blocklist, getResp.Mode)
	assert.Equal(t, []string{"gpt-4o"}, getResp.Models)

	// PUT invalid mode
	invalidBody := bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(invalidBody).Encode(map[string]interface{}{"mode": "bogus", "models": []string{}}))
	req3 := httptest.NewRequest(http.MethodPut, "/api/policy", invalidBody)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusBadRequest, rec3.Code)
}

func TestGetPolicyModels(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()
	h := NewHandler(s)

	ctx := context.Background()
	now := time.Now().UTC()
	// Insert some requests with models
	s.InsertRequest(ctx, store.RequestRecord{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.openai.com", Model: "gpt-4o", Status: 200, LatencyMS: 100})
	s.InsertRequest(ctx, store.RequestRecord{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.openai.com", Model: "claude-3.5-sonnet", Status: 200, LatencyMS: 100})
	s.InsertRequest(ctx, store.RequestRecord{Timestamp: now, Endpoint: "chat", Method: "POST", Path: "/chat", UpstreamHost: "api.openai.com", Model: "gpt-4o", Status: 200, LatencyMS: 100})

	req := httptest.NewRequest(http.MethodGet, "/api/policy/models", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var models []string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&models))
	expected := []string{"claude-3.5-sonnet", "gpt-4o"}
	// The order from SQLite DISTINCT with ORDER BY should be alphabetical
	assert.Equal(t, expected, models)
}
