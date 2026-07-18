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

	out, err := MakeUpstreamRequest(in, "api.githubcopilot.com", []byte(`{"model":"gpt-4o"}`))
	if err != nil {
		t.Fatal(err)
	}

	if out.URL.Scheme != "https" {
		t.Fatalf("scheme = %q, want https", out.URL.Scheme)
	}
	if out.URL.Host != "api.githubcopilot.com" {
		t.Fatalf("host = %q, want api.githubcopilot.com", out.URL.Host)
	}
	if out.URL.RequestURI() != "/chat/completions?x=1" {
		t.Fatalf("request URI = %q", out.URL.RequestURI())
	}
	if out.Host != "api.githubcopilot.com" {
		t.Fatalf("out.Host = %q, want api.githubcopilot.com", out.Host)
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
	h := NewHandler(log.NewWriterWithFormat(io.Discard, log.FormatHuman))
	h.SetUpstream("api.githubcopilot.com")
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
}

func TestHandlerForwardToUpstream(t *testing.T) {
	// Unknown paths are forwarded to the configured upstream.
	h := NewHandler(log.NewWriterWithFormat(io.Discard, log.FormatHuman))
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.githubcopilot.com" {
			t.Fatalf("host = %q, want api.githubcopilot.com", req.URL.Host)
		}
		if req.URL.Path != "/nope" {
			t.Fatalf("path = %q, want /nope", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"forwarded":true}`)),
		}, nil
	})})

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/nope", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != `{"forwarded":true}` {
		t.Fatalf("body = %q, want forwarded response", rr.Body.String())
	}
}

func TestHandlerPersistsUsageMissingWhenNoUsage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "test-project")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			// No usage field — should trigger UsageMissing
			Body: io.NopCloser(strings.NewReader(`{"model":"gpt-4o","id":"chatcmpl-123","choices":[]}`)),
		}, nil
	})})

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Count usage_missing rows directly
	ctx := context.Background()
	count, err := st.CountUsageMissing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("usage_missing count = %d, want 1", count)
	}

	// Verify stats show the model but zero tokens
	stats, err := st.Stats(ctx, store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("stats count = %d, want 1: %#v", len(stats), stats)
	}
	if stats[0].Model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", stats[0].Model)
	}
	if stats[0].TotalTokens != 0 {
		t.Fatalf("total_tokens = %d, want 0", stats[0].TotalTokens)
	}
	if !stats[0].UsageMissing {
		t.Fatal("expected UsageMissing = true")
	}
}

func TestHandlerPersistsSSEUsage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "test-project")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.githubcopilot.com" {
			t.Fatalf("host = %q, want api.githubcopilot.com", req.URL.Host)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":7,\"prompt_tokens_details\":{\"cached_tokens\":2},\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})})

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
	for _, want := range []string{"gpt-4o", "200", "⬆ 7", "⬇ 3"} {
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
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "test-project")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"model":"gpt-4o","usage":{"prompt_tokens":7,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens":3,"total_tokens":10}}`)),
		}, nil
	})})

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
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":"` + sensitive + `"}`)),
		}, nil
	})})

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
	// Row should now be persisted with usage_missing=true
	stats, err := st.Stats(context.Background(), store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 persisted row (usage_missing), got: %#v", stats)
	}
	if !stats[0].UsageMissing {
		t.Fatal("expected UsageMissing = true for error body without usage")
	}
	if stats[0].TotalTokens != 0 {
		t.Fatalf("expected 0 total tokens, got %d", stats[0].TotalTokens)
	}
}

func TestHandlerWritesUsageDebugRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	usageDebug, err := OpenUsageDebugLogger(path)
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	h := NewHandlerWithStoreAndUsageDebug(log.NewWriterWithFormat(&logs, log.FormatHuman), nil, "", usageDebug)
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
				"X-Request-Id": {"abc"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-5-mini\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})})

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
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
		}, nil
	})})

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/agents/swe/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Zero-usage agent routes are persisted as control_plane but excluded from
	// usage stats because they are not model-generation traffic.
	ctx := context.Background()
	exportRows, err := st.ExportRequests(ctx, time.Time{}, time.Time{}, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(exportRows) != 1 {
		t.Fatalf("expected 1 persisted row, got: %#v", exportRows)
	}
	if exportRows[0].EndpointKind != store.EndpointKindControlPlane {
		t.Fatalf("expected endpoint_kind = control_plane, got %q", exportRows[0].EndpointKind)
	}
	missing, err := st.CountUsageMissing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if missing != 1 {
		t.Fatalf("expected 1 usage_missing row, got %d", missing)
	}

	stats, err := st.Stats(ctx, store.StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected zero usage stats, got: %#v", stats)
	}
}

func TestStructuredJSONLogLineEmitted(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandler(log.NewWriterWithFormat(&logs, log.FormatJSON))
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)),
		}, nil
	})})

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Parse the last line of the log output as JSON
	logLines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	var lastLine string
	for i := len(logLines) - 1; i >= 0; i-- {
		if logLines[i] != "" {
			lastLine = logLines[i]
			break
		}
	}
	if lastLine == "" {
		t.Fatalf("no log lines found in:\n%s", logs.String())
	}

	var entry log.RequestLog
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, lastLine)
	}

	// Validate required fields
	if entry.RequestID == 0 {
		t.Fatal("missing request_id")
	}
	if entry.Method != "POST" {
		t.Fatalf("method = %q, want POST", entry.Method)
	}
	if entry.Path == "" {
		t.Fatal("missing path")
	}
	if entry.Upstream == "" {
		t.Fatal("missing upstream")
	}
	if entry.Model == "" {
		t.Fatal("missing model")
	}
	if entry.Status != http.StatusOK {
		t.Fatalf("status = %d, want %d", entry.Status, http.StatusOK)
	}
	if !entry.TokensCaptured {
		t.Fatal("expected tokens_captured = true")
	}
}

func TestHealthEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/_health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	// Validate required fields
	if resp["status"] != "ok" {
		t.Fatalf("status = %q, want ok", resp["status"])
	}
	if _, ok := resp["uptime_seconds"]; !ok {
		t.Fatal("missing uptime_seconds")
	}
	if _, ok := resp["requests_total"]; !ok {
		t.Fatal("missing requests_total")
	}
	if _, ok := resp["db_size_bytes"]; !ok {
		t.Fatal("missing db_size_bytes")
	}
}

func TestHandlerPrefersRequestModelOverResponseModel(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o-mini-2024-07-18\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
		}, nil
	})})

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
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})})

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
	h := NewHandler(log.NewWriterWithFormat(&logs, log.FormatHuman))
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"total_tokens":100}}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})})

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
	h := NewHandlerWithStore(log.NewWriterWithFormat(&logs, log.FormatHuman), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})})

	// 2. First request — blocked (warms cache)
	body := strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/chat/completions", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// 3. Expire the cache
	h.ExpirePolicyCache()

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

func TestHandlerClassifiesHelperEndpoints(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer st.Close()

	h := NewHandlerWithStore(log.NewWriterWithFormat(io.Discard, log.FormatHuman), st, "test-project")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		}, nil
	})})

	for _, path := range []string{"/models", "/agents"} {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733"+path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, "path %s", path)
	}

	// All persisted rows should be control_plane.
	exportRows, err := st.ExportRequests(ctx, time.Time{}, time.Time{}, "", "", "")
	require.NoError(t, err)
	require.Len(t, exportRows, 2, "helper endpoints should be persisted")
	for i, row := range exportRows {
		assert.Equal(t, store.EndpointKindControlPlane, row.EndpointKind, "row %d", i)
	}

	// Stats should exclude helper endpoints.
	stats, err := st.Stats(ctx, store.StatsFilter{})
	require.NoError(t, err)
	require.Empty(t, stats, "helper endpoints should not appear in usage stats")
}
