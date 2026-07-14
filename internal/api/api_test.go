package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
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

func TestPutPolicyInvalidModels(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer s.Close()
	h := NewHandler(s)

	// Empty string in models
	body := bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(body).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o", ""}}))
	req := httptest.NewRequest(http.MethodPut, "/api/policy", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "empty")

	// Whitespace-only string in models
	body = bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(body).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"  ", "gpt-4o"}}))
	req = httptest.NewRequest(http.MethodPut, "/api/policy", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Duplicate models
	body = bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(body).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o", "gpt-4o"}}))
	req = httptest.NewRequest(http.MethodPut, "/api/policy", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "duplicate")

	// Models with whitespace around them should be trimmed and pass
	body = bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(body).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"  gpt-4o  ", "claude"}}))
	req = httptest.NewRequest(http.MethodPut, "/api/policy", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var p policy.Policy
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
	assert.Equal(t, []string{"gpt-4o", "claude"}, p.Models)

	// Whitespace trimming leading to duplicate should fail
	body = bytes.NewBuffer(nil)
	require.NoError(t, json.NewEncoder(body).Encode(policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o", "  gpt-4o  "}}))
	req = httptest.NewRequest(http.MethodPut, "/api/policy", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSessionsCursorPagination(t *testing.T) {
	st, server := newAPITestServer(t)
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		insertAPISession(t, st, base.Add(time.Duration(i)*time.Hour), "project")
	}

	var firstPage []store.SessionStats
	response, err := server.Client().Get(server.URL + "/api/sessions?limit=2")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&firstPage))
	response.Body.Close()
	require.Len(t, firstPage, 2)

	cursor := firstPage[len(firstPage)-1]
	query := url.Values{"limit": {"2"}, "cursor": {cursor.StartedAt.Format(time.RFC3339Nano)}, "cursor_id": {strconv.FormatInt(cursor.ID, 10)}}
	var secondPage []store.SessionStats
	response, err = server.Client().Get(server.URL + "/api/sessions?" + query.Encode())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&secondPage))
	response.Body.Close()
	require.Len(t, secondPage, 2)

	seen := make(map[int64]bool)
	for _, session := range append(firstPage, secondPage...) {
		assert.False(t, seen[session.ID], "session %d appeared on more than one page", session.ID)
		seen[session.ID] = true
	}

	// A third page is needed because two pages of two items cannot contain all five sessions.
	cursor = secondPage[len(secondPage)-1]
	query.Set("cursor", cursor.StartedAt.Format(time.RFC3339Nano))
	query.Set("cursor_id", strconv.FormatInt(cursor.ID, 10))
	var thirdPage []store.SessionStats
	response, err = server.Client().Get(server.URL + "/api/sessions?" + query.Encode())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&thirdPage))
	response.Body.Close()
	require.Len(t, thirdPage, 1)
	for _, session := range thirdPage {
		assert.False(t, seen[session.ID], "session %d appeared on more than one page", session.ID)
		seen[session.ID] = true
	}
	assert.Len(t, seen, 5)
}

func TestSessionsCursorInvalidParams(t *testing.T) {
	_, server := newAPITestServer(t)
	for _, path := range []string{
		"/api/sessions?cursor=2026-07-01T10:00:00Z",
		"/api/sessions?cursor_id=1",
		"/api/sessions?cursor=not-a-time&cursor_id=1",
		"/api/sessions?cursor=2026-07-01T10:00:00Z&cursor_id=not-a-number",
	} {
		t.Run(path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + path)
			require.NoError(t, err)
			defer response.Body.Close()
			assert.Equal(t, http.StatusBadRequest, response.StatusCode)

			var body map[string]string
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			assert.NotEmpty(t, body["error"])
		})
	}
}

func TestSessionsCount(t *testing.T) {
	st, server := newAPITestServer(t)
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		insertAPISession(t, st, base.Add(time.Duration(i)*time.Hour), "project")
	}

	for _, test := range []struct {
		path  string
		count int
	}{
		{"/api/sessions/count", 3},
		{"/api/sessions/count?project=nonexistent", 0},
	} {
		t.Run(test.path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + test.path)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)

			var body struct {
				Count int `json:"count"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			assert.Equal(t, test.count, body.Count)
		})
	}
}

func TestDistinctProjects(t *testing.T) {
	st, server := newAPITestServer(t)
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i, project := range []string{"alpha", "beta", "alpha"} {
		insertAPISession(t, st, base.Add(time.Duration(i)*time.Hour), project)
	}

	response, err := server.Client().Get(server.URL + "/api/sessions/distinct-projects")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var projects []string
	require.NoError(t, json.NewDecoder(response.Body).Decode(&projects))
	assert.Equal(t, []string{"alpha", "beta"}, projects)
}

func TestAnomaliesEndpoint(t *testing.T) {
	st, server := newAPITestServer(t)
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for i, anomaly := range []store.AnomalyRecord{
		{Category: "unrouted_path", Severity: "warn"},
		{Category: "parse_error", Severity: "info"},
		{Category: "auth_missing", Severity: "error"},
	} {
		require.NoError(t, st.WriteAnomaly(context.Background(), store.AnomalyRecord{
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Category:  anomaly.Category,
			Severity:  anomaly.Severity,
		}))
	}

	for _, test := range []struct {
		path       string
		categories []string
	}{
		{"/api/anomalies", []string{"auth_missing", "parse_error", "unrouted_path"}},
		{"/api/anomalies?category=parse_error", []string{"parse_error"}},
		{"/api/anomalies?severity=error", []string{"auth_missing"}},
		{"/api/anomalies?category=nonexistent", []string{}},
	} {
		t.Run(test.path, func(t *testing.T) {
			response, err := server.Client().Get(server.URL + test.path)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)

			var anomalies []store.Anomaly
			require.NoError(t, json.NewDecoder(response.Body).Decode(&anomalies))
			categories := make([]string, len(anomalies))
			for i, anomaly := range anomalies {
				categories[i] = anomaly.Category
			}
			assert.Equal(t, test.categories, categories)
		})
	}

	_, emptyServer := newAPITestServer(t)
	response, err := emptyServer.Client().Get(emptyServer.URL + "/api/anomalies")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var anomalies []store.Anomaly
	require.NoError(t, json.NewDecoder(response.Body).Decode(&anomalies))
	assert.Empty(t, anomalies)
}

func TestSanitizedErrorResponses(t *testing.T) {
	st, server := newAPITestServer(t)
	require.NoError(t, st.Close())

	response, err := server.Client().Get(server.URL + "/api/sessions")
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	assert.Equal(t, "internal server error", body["error"])
	assert.NotContains(t, body["error"], "database is closed")
	assert.NotContains(t, body["error"], "sqlite")
}

func newAPITestServer(t *testing.T) (*store.Store, *httptest.Server) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	server := httptest.NewServer(NewHandler(st))
	t.Cleanup(server.Close)
	return st, server
}

func insertAPISession(t *testing.T, st *store.Store, timestamp time.Time, project string) {
	t.Helper()
	require.NoError(t, st.InsertRequest(context.Background(), store.RequestRecord{
		Timestamp: timestamp, Endpoint: "chat", Method: http.MethodPost, Path: "/chat/completions",
		UpstreamHost: "api.githubcopilot.com", Model: "gpt-4o", Status: http.StatusOK, Project: project,
	}))
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
