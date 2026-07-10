package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"copilot-monitoring/internal/compression/headroom"
	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

type recordingCompressor struct {
	result headroom.CompressionResult
	err    error
	calls  int
	last   headroom.CompressionRequest
}

func (c *recordingCompressor) Compress(_ context.Context, req headroom.CompressionRequest) (headroom.CompressionResult, error) {
	c.calls++
	c.last = req
	return c.result, c.err
}

type compressionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f compressionRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func compressionTestRouter() *Router {
	return NewRouter(&ProxyConfig{
		Routes: []RouteConfig{
			{Path: "/chat/completions", UpstreamHost: "provider.example.com", Capture: "usage"},
		},
	})
}

func TestHandlerCompressionReplacesMessagesAndPreservesProviderHeaders(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandlerWithRouter(log.NewWriterWithFormat(&logs, log.FormatHuman), nil, "", nil, compressionTestRouter())
	fake := &recordingCompressor{result: headroom.CompressionResult{
		Messages:         json.RawMessage(`[{"role":"user","content":"compressed"}]`),
		OriginalTokens:   20,
		CompressedTokens: 5,
		TokensSaved:      15,
		CompressionRatio: 0.25,
	}}
	h.ConfigureCompression(fake, false)

	var providerBody []byte
	var providerAuth string
	h.client = &http.Client{Transport: compressionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		providerBody, _ = io.ReadAll(req.Body)
		providerAuth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)),
		}, nil
	})}

	original := `{"model":"gpt-4o","messages":[{"role":"user","content":"original"}],"stream":false,"temperature":0.2,"tools":[{"type":"function"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer provider-secret")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("compressor calls = %d, want 1", fake.calls)
	}
	if providerAuth != "Bearer provider-secret" {
		t.Fatalf("provider authorization = %q", providerAuth)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(providerBody, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["messages"]) != `[{"role":"user","content":"compressed"}]` {
		t.Fatalf("provider messages = %s", got["messages"])
	}
	for _, field := range []string{"model", "stream", "temperature", "tools"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("provider body lost %q: %s", field, providerBody)
		}
	}
	if strings.Contains(logs.String(), "%!") {
		t.Fatalf("formatting artifact in compression logs: %s", logs.String())
	}
}

func TestHandlerCompressionFailOpenForwardsOriginalBody(t *testing.T) {
	h := NewHandlerWithRouter(log.Disabled(), nil, "", nil, compressionTestRouter())
	fake := &recordingCompressor{err: errors.New("synthetic Headroom outage")}
	h.ConfigureCompression(fake, false)
	var providerBody []byte
	h.client = &http.Client{Transport: compressionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		providerBody, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})}

	original := `{"model":"gpt-4o","messages":[{"role":"user","content":"original"}]}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if string(providerBody) != original {
		t.Fatalf("provider body changed on fail-open: got %s want %s", providerBody, original)
	}
}

func TestHandlerCompressionRequiredReturns502WithoutProviderCall(t *testing.T) {
	h := NewHandlerWithRouter(log.Disabled(), nil, "", nil, compressionTestRouter())
	fake := &recordingCompressor{err: errors.New("synthetic Headroom outage")}
	h.ConfigureCompression(fake, true)
	providerCalls := 0
	h.client = &http.Client{Transport: compressionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		providerCalls++
		return nil, errors.New("provider must not be called")
	})}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if rec.Body.String() != "request compression failed\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if providerCalls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerCalls)
	}
}

func TestHandlerCompressionUnsupportedEnvelopeBypassesEvenWhenRequired(t *testing.T) {
	h := NewHandlerWithRouter(log.Disabled(), nil, "", nil, compressionTestRouter())
	fake := &recordingCompressor{result: headroom.CompressionResult{
		Messages:         json.RawMessage(`[]`),
		OriginalTokens:   1,
		CompressedTokens: 1,
		CompressionRatio: 1,
	}}
	h.ConfigureCompression(fake, true)
	var providerBody []byte
	h.client = &http.Client{Transport: compressionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		providerBody, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})}

	original := `{"model":"gpt-4o","messages":{}}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("compressor calls = %d, want 0", fake.calls)
	}
	if string(providerBody) != original {
		t.Fatalf("provider body = %s, want original %s", providerBody, original)
	}
}

func TestHandlerPolicyRunsBeforeCompression(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/store.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetPolicy(context.Background(), &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}}); err != nil {
		t.Fatal(err)
	}

	h := NewHandlerWithRouter(log.Disabled(), st, "", nil, compressionTestRouter())
	fake := &recordingCompressor{err: errors.New("compressor must not be called")}
	h.ConfigureCompression(fake, true)
	h.client = &http.Client{Transport: compressionRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("provider must not be called")
	})}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"secret"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("compressor calls = %d, want 0", fake.calls)
	}
}

func TestCompressionEligiblePaths(t *testing.T) {
	fake := &recordingCompressor{}
	base := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7733/chat/completions", nil)
	base.Header.Set("Content-Type", "application/json; charset=utf-8")
	route := Route{Endpoint: "chat"}
	tests := []struct {
		name string
		edit func(*http.Request, *Route)
		want bool
	}{
		{name: "chat", want: true},
		{name: "openai chat", edit: func(r *http.Request, _ *Route) { r.URL.Path = "/v1/chat/completions" }, want: true},
		{name: "get", edit: func(r *http.Request, _ *Route) { r.Method = http.MethodGet }, want: false},
		{name: "wrong content type", edit: func(r *http.Request, _ *Route) { r.Header.Set("Content-Type", "text/plain") }, want: false},
		{name: "other path", edit: func(r *http.Request, _ *Route) { r.URL.Path = "/v1/messages" }, want: false},
		{name: "local", edit: func(_ *http.Request, route *Route) { route.Local = true }, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := base.Clone(base.Context())
			routeCopy := route
			if tt.edit != nil {
				tt.edit(r, &routeCopy)
			}
			if got := compressionEligible(r, routeCopy, fake); got != tt.want {
				t.Fatalf("compressionEligible = %t, want %t", got, tt.want)
			}
		})
	}
}
