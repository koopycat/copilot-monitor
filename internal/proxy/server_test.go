package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

func TestMakeUpstreamRequestPreservesAuthAndRewritesTarget(t *testing.T) {
	in := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions?x=1", strings.NewReader(`{"model":"gpt-4o"}`))
	in.Header.Set("Authorization", "Bearer secret")
	in.Header.Set("Content-Type", "application/json")
	in.Header.Set("Connection", "keep-alive")

	route, ok := NewRouter(nil).Match("/chat/completions")
	if !ok {
		t.Fatal("missing chat route")
	}
	out, err := MakeUpstreamRequest(in, route, []byte(`{"model":"gpt-4o"}`))
	if err != nil {
		t.Fatal(err)
	}

	if out.URL.Scheme != "https" {
		t.Fatalf("scheme = %q, want https", out.URL.Scheme)
	}
	if out.URL.Host != GitHubCopilotAPIHost {
		t.Fatalf("host = %q, want %q", out.URL.Host, GitHubCopilotAPIHost)
	}
	if out.URL.RequestURI() != "/chat/completions?x=1" {
		t.Fatalf("request URI = %q", out.URL.RequestURI())
	}
	if out.Host != GitHubCopilotAPIHost {
		t.Fatalf("out.Host = %q, want %q", out.Host, GitHubCopilotAPIHost)
	}
	if got := out.Header.Get("Authorization"); got != "Bearer secret" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := out.Header.Get("Connection"); got != "" {
		t.Fatalf("Connection header was not stripped: %q", got)
	}
	if got := out.Header.Get("Accept-Encoding"); got != "identity" {
		t.Fatalf("Accept-Encoding = %q, want identity", got)
	}
}

func TestStripHopByHopHeadersAlsoStripsConnectionTokens(t *testing.T) {
	headers := http.Header{
		"Connection":        {"x-remove, keep-alive"},
		"X-Remove":          {"bad"},
		"Keep-Alive":        {"timeout=5"},
		"Authorization":     {"Bearer secret"},
		"X-Should-Remain":   {"ok"},
		"Transfer-Encoding": {"chunked"},
	}

	got := StripHopByHopHeaders(headers)
	for _, name := range []string{"Connection", "X-Remove", "Keep-Alive", "Transfer-Encoding"} {
		if got.Get(name) != "" {
			t.Fatalf("%s was not stripped: %#v", name, got.Values(name))
		}
	}
	if got.Get("Authorization") != "Bearer secret" {
		t.Fatalf("Authorization = %q", got.Get("Authorization"))
	}
	if got.Get("X-Should-Remain") != "ok" {
		t.Fatalf("X-Should-Remain = %q", got.Get("X-Should-Remain"))
	}
}

func TestHandlerPing(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandler(log.NewWriter(&logs))
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/_ping", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "OK" {
		t.Fatalf("body = %q, want OK", string(body))
	}
	if !strings.Contains(logs.String(), "endpoint=ping") {
		t.Fatalf("logs missing endpoint=ping: %s", logs.String())
	}
}

func TestHandlerUnknownPath(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandler(log.NewWriter(&logs))
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/nope", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
	if !strings.Contains(logs.String(), "route=unknown") {
		t.Fatalf("logs missing route=unknown: %s", logs.String())
	}
}

func TestHandlerPersistsSSEUsage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "test-project")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != GitHubCopilotAPIHost {
			t.Fatalf("host = %q, want %q", req.URL.Host, GitHubCopilotAPIHost)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":7,\"prompt_tokens_details\":{\"cached_tokens\":2},\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	if stats[0].Model != "gpt-4o" || stats[0].PromptTokens != 7 || stats[0].CachedInputTokens != 2 || stats[0].CompletionTokens != 3 || stats[0].TotalTokens != 10 {
		t.Fatalf("stats[0] = %#v", stats[0])
	}
	if strings.Contains(logs.String(), "%!") {
		t.Fatalf("response log has formatting artifact: %s", logs.String())
	}
	for _, want := range []string{"usage_seen=true", "prompt_tokens=7", "total=10"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("response log missing %q: %s", want, logs.String())
		}
	}
}

func TestHandlerPersistsJSONUsage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "test-project")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"model":"gpt-4o","usage":{"prompt_tokens":7,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens":3,"total_tokens":10}}`)),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":false}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	if stats[0].Model != "gpt-4o" || stats[0].PromptTokens != 7 || stats[0].CachedInputTokens != 2 || stats[0].CompletionTokens != 3 || stats[0].TotalTokens != 10 {
		t.Fatalf("stats[0] = %#v", stats[0])
	}
}

func TestHandlerDoesNotRetainUpstreamErrorBody(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const sensitive = "do not keep this prompt text"
	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":"` + sensitive + `"}`)),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":false}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), sensitive) {
		t.Fatalf("proxied response body = %q, want upstream body", rr.Body.String())
	}
	if strings.Contains(logs.String(), sensitive) {
		t.Fatalf("logs retained upstream body: %s", logs.String())
	}
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected no persisted usage row for error body without usage, got: %#v", stats)
	}
}

func TestHandlerWritesUsageDebugRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	usageDebug, err := OpenUsageDebugLogger(path)
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	h := NewHandlerWithStoreAndUsageDebug(log.NewWriter(&logs), nil, "", usageDebug)
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
				"X-Request-Id": {"abc"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-5-mini\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-5-mini","stream":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if err := usageDebug.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing debug record")
	}
	var record UsageDebugRecord
	if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.RequestModel != "gpt-5-mini" || record.ResponseModel != "gpt-5-mini" || !record.UsageDetected {
		t.Fatalf("record = %#v", record)
	}
	if record.ResponseHeaders["X-Request-Id"][0] != "abc" {
		t.Fatalf("headers = %#v", record.ResponseHeaders)
	}
}

func TestHandlerDoesNotPersistZeroUsageAgentRoutes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/agents/swe/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected no persisted rows for zero-usage agent route, got: %#v", stats)
	}
}

func TestHandlerPrefersRequestModelOverResponseModel(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o-mini-2024-07-18\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"claude-sonnet-4","stream":true}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	if stats[0].Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want request model", stats[0].Model)
	}
}

func TestHandlerRoutesByModel(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Create config with two routes on same path, different models
	cfg := &ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/v1/chat/completions", UpstreamHost: "openai.example.com", Capture: "usage", Models: []string{"gpt-4o"}},
			{Path: "/v1/chat/completions", UpstreamHost: "anthropic.example.com", Capture: "usage", Models: []string{"claude-*"}},
		},
	}
	router := NewRouter(cfg)

	var logs bytes.Buffer
	h := NewHandlerWithRouter(log.NewWriter(&logs), st, "", nil, router)

	var capturedHost string
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedHost = req.URL.Host
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)),
		}, nil
	})}

	// gpt-4o should go to OpenAI upstream
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if capturedHost != "openai.example.com" {
		t.Fatalf("host = %q, want openai.example.com", capturedHost)
	}

	// claude-sonnet should go to Anthropic upstream
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-sonnet-4","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if capturedHost != "anthropic.example.com" {
		t.Fatalf("host = %q, want anthropic.example.com", capturedHost)
	}
}

func TestPolicyBlocklistBlocksModel(t *testing.T) {
	// 1. Open store
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer st.Close()

	// 2. Set blocklist policy
	ctx := context.Background()
	err = st.SetPolicy(ctx, &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}})
	require.NoError(t, err)

	// 3. Create handler with store
	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	// 4. Send blocked model request
	body := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/chat/completions", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// 5. Assert 403 with JSON error
	assert.Equal(t, http.StatusForbidden, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "model_blocked", resp["error"])
	assert.Equal(t, "gpt-4o", resp["model"])

	// 6. Verify blocked request persisted
	models, err := st.DistinctModels(ctx)
	require.NoError(t, err)
	assert.Contains(t, models, "gpt-4o")

	// 7. Send unblocked model — should pass through
	body2 := strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)
	req2 := httptest.NewRequest("POST", "/chat/completions", body2)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestPolicyNotAppliedWhenNoStore(t *testing.T) {
	// Handler without store — policy evaluation skipped
	var logs bytes.Buffer
	h := NewHandler(log.NewWriter(&logs))
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"total_tokens":100}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	body := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/chat/completions", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPolicyCacheRefreshOnExpiry(t *testing.T) {
	// 1. Set up store with blocklist
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer st.Close()
	ctx := context.Background()
	err = st.SetPolicy(ctx, &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}})
	require.NoError(t, err)

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriter(&logs), st, "")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	// 2. First request — blocked (warms cache)
	body := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/chat/completions", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// 3. Expire the cache
	h.policyUntil = time.Now().Add(-1 * time.Second)

	// 4. Change policy to allow_all
	err = st.SetPolicy(ctx, &policy.Policy{Mode: policy.AllowAll})
	require.NoError(t, err)

	// 5. Next request — allowed (cache refreshed)
	body2 := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req2 := httptest.NewRequest("POST", "/chat/completions", body2)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
