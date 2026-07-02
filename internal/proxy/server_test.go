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

	"copilot-monitoring/internal/store"
)

func TestMakeUpstreamRequestPreservesAuthAndRewritesTarget(t *testing.T) {
	in := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions?x=1", strings.NewReader(`{"model":"gpt-4o"}`))
	in.Header.Set("Authorization", "Bearer secret")
	in.Header.Set("Content-Type", "application/json")
	in.Header.Set("Connection", "keep-alive")

	route, ok := RoutePath("/chat/completions")
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
	h := NewHandler(&logs)
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
	h := NewHandler(&logs)
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
	h := NewHandlerWithStore(&logs, st, "test-project")
	h.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != GitHubCopilotAPIHost {
			t.Fatalf("host = %q, want %q", req.URL.Host, GitHubCopilotAPIHost)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": {"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader("data: {\"model\":\"gpt-4o\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n")),
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
	if stats[0].Model != "gpt-4o" || stats[0].PromptTokens != 7 || stats[0].CompletionTokens != 3 || stats[0].TotalTokens != 10 {
		t.Fatalf("stats[0] = %#v", stats[0])
	}
}

func TestHandlerWritesUsageDebugRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	usageDebug, err := OpenUsageDebugLogger(path)
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	h := NewHandlerWithStoreAndUsageDebug(&logs, nil, "", usageDebug)
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
	if record.RequestModel != "gpt-5-mini" || record.ResponseModel != "gpt-5-mini" || !record.UsageDetected || len(record.UsageObjects) != 1 {
		t.Fatalf("record = %#v", record)
	}
	if record.ResponseHeaders["X-Request-Id"][0] != "abc" {
		t.Fatalf("headers = %#v", record.ResponseHeaders)
	}
}

func TestHandlerPrefersRequestModelOverResponseModel(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	var logs bytes.Buffer
	h := NewHandlerWithStore(&logs, st, "")
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
