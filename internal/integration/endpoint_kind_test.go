//go:build testonly

package integration

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"copilot-monitoring/internal/api"
	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_HelperEndpointsExcludedFromUsageViews(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/store.db")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	fakeUpstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/models", "/agents":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
		}
	}))
	t.Cleanup(fakeUpstream.Close)

	u, err := url.Parse(fakeUpstream.URL)
	require.NoError(t, err)

	h := proxy.NewHandlerWithStore(log.Disabled(), st, "")
	h.SetUpstream(u.Host)
	h.SetTestClient(&http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}})

	// Capture a helper request and an inference request.
	modelsReq := httptest.NewRequest(http.MethodGet, "/models", nil)
	modelsReq.Header.Set("Authorization", "Bearer test-token")
	modelsRec := httptest.NewRecorder()
	h.ServeHTTP(modelsRec, modelsReq)
	require.Equal(t, http.StatusOK, modelsRec.Code)

	chatReq := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.Header.Set("Authorization", "Bearer test-token")
	chatRec := httptest.NewRecorder()
	h.ServeHTTP(chatRec, chatReq)
	require.Equal(t, http.StatusOK, chatRec.Code)

	apiHandler := api.NewHandler(st)

	// /api/stats
	statsRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(statsRec, httptest.NewRequest(http.MethodGet, "/api/stats?since=1d", nil))
	require.Equal(t, http.StatusOK, statsRec.Code)
	var stats []store.ModelStats
	require.NoError(t, json.Unmarshal(statsRec.Body.Bytes(), &stats))
	require.Len(t, stats, 1)
	assert.Equal(t, "gpt-4o", stats[0].Model)
	assert.Equal(t, 1, stats[0].Requests)

	// /api/cost
	costRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(costRec, httptest.NewRequest(http.MethodGet, "/api/cost?since=1d", nil))
	require.Equal(t, http.StatusOK, costRec.Code)
	var costResponse struct {
		TotalUSD float64 `json:"total_usd"`
		Rows     []struct {
			Model    string  `json:"model"`
			Requests int     `json:"requests"`
			TotalUSD float64 `json:"total_usd"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(costRec.Body.Bytes(), &costResponse))
	require.Len(t, costResponse.Rows, 1)
	assert.Equal(t, "gpt-4o", costResponse.Rows[0].Model)

	// /api/today
	todayRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(todayRec, httptest.NewRequest(http.MethodGet, "/api/today", nil))
	require.Equal(t, http.StatusOK, todayRec.Code)
	var today []store.ModelStats
	require.NoError(t, json.Unmarshal(todayRec.Body.Bytes(), &today))
	require.Len(t, today, 1)
	assert.Equal(t, "gpt-4o", today[0].Model)

	// /api/stats/timeline
	timelineRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(timelineRec, httptest.NewRequest(http.MethodGet, "/api/stats/timeline?since=1d&granularity=day", nil))
	require.Equal(t, http.StatusOK, timelineRec.Code)
	var timeline []store.TimelineBucket
	require.NoError(t, json.Unmarshal(timelineRec.Body.Bytes(), &timeline))
	require.Len(t, timeline, 1)
	assert.Equal(t, "gpt-4o", timeline[0].Model)

	// Export retains the helper row and includes endpoint_kind.
	exportRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(exportRec, httptest.NewRequest(http.MethodGet, "/api/export?since=1d", nil))
	require.Equal(t, http.StatusOK, exportRec.Code)
	body := exportRec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.Len(t, lines, 3) // header + 2 rows
	assert.Contains(t, lines[0], "endpoint_kind")
	assert.Contains(t, body, store.EndpointKindControlPlane)
	assert.Contains(t, body, store.EndpointKindInference)
}

func TestIntegration_ExportIncludesEndpointKind(t *testing.T) {
	ctx := context.Background()

	st, err := store.Open(t.TempDir() + "/store.db")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	now := time.Now().UTC()
	require.NoError(t, st.InsertRequest(ctx, store.RequestRecord{
		Timestamp: now, Endpoint: "/models", Method: "GET", Path: "/models", UpstreamHost: "api.example.com", Status: 200, EndpointKind: store.EndpointKindControlPlane,
	}))
	require.NoError(t, st.InsertRequest(ctx, store.RequestRecord{
		Timestamp: now, Endpoint: "/chat/completions", Method: "POST", Path: "/chat/completions", UpstreamHost: "api.example.com", Model: "gpt-4o", Status: 200, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, EndpointKind: store.EndpointKindInference,
	}))

	apiHandler := api.NewHandler(st)
	exportRec := httptest.NewRecorder()
	apiHandler.ServeHTTP(exportRec, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	require.Equal(t, http.StatusOK, exportRec.Code)

	body := exportRec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.Len(t, lines, 3)
	header := lines[0]
	assert.Contains(t, header, "endpoint_kind")

	var foundControlPlane, foundInference bool
	for _, line := range lines[1:] {
		if strings.Contains(line, store.EndpointKindControlPlane) {
			foundControlPlane = true
		}
		if strings.Contains(line, store.EndpointKindInference) {
			foundInference = true
		}
	}
	assert.True(t, foundControlPlane, "export should include control_plane row")
	assert.True(t, foundInference, "export should include inference row")
}
