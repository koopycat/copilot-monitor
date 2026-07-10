package headroom

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientCompress(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/compress" {
			t.Errorf("path = %q, want /v1/compress", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		for _, name := range []string{"Authorization", "Cookie", "X-Provider-Key"} {
			if got := r.Header.Get(name); got != "" {
				t.Errorf("%s leaked to Headroom: %q", name, got)
			}
		}

		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(request) != 2 {
			t.Errorf("request fields = %v, want only model and messages", request)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{
			"messages":[{"role":"user","content":"short"}],
			"tokens_before":100,
			"tokens_after":25,
			"tokens_saved":75,
			"compression_ratio":0.25,
			"transforms_applied":["test"],
			"ccr_hashes":["abc"]
		}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Compress(context.Background(), CompressionRequest{
		Model:    "gpt-4o",
		Messages: json.RawMessage(`[{"role":"user","content":"a long synthetic message"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OriginalTokens != 100 || result.CompressedTokens != 25 || result.TokensSaved != 75 {
		t.Fatalf("token metrics = %#v", result)
	}
	if result.CompressionRatio != 0.25 {
		t.Fatalf("compression ratio = %v, want 0.25", result.CompressionRatio)
	}
	if string(result.Messages) != `[{"role":"user","content":"short"}]` {
		t.Fatalf("messages = %s", result.Messages)
	}
}

func TestNewClientRejectsUnsafeEndpoint(t *testing.T) {
	t.Parallel()

	tests := []string{
		"https://127.0.0.1:8787/v1/compress",
		"http://192.0.2.1:8787/v1/compress",
		"http://user@127.0.0.1:8787/v1/compress",
		"http://127.0.0.1:8787/v1/compress?debug=1",
		"http://127.0.0.1:8787/compress",
		"http://127.0.0.1/v1/compress",
	}
	for _, endpoint := range tests {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(endpoint, ClientOptions{}); err == nil {
				t.Fatalf("NewClient(%q) succeeded, want error", endpoint)
			}
		})
	}
}

func TestClientDoesNotExposeErrorBody(t *testing.T) {
	t.Parallel()

	const sensitive = "synthetic prompt echoed by service"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, sensitive, http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Compress(context.Background(), validRequest())
	if err == nil {
		t.Fatal("Compress succeeded, want error")
	}
	if strings.Contains(err.Error(), sensitive) {
		t.Fatalf("error exposed response body: %q", err)
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want status 503", err)
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var redirectedCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirectedCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
			return
		}
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Compress(context.Background(), validRequest())
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("error = %v, want status 307", err)
	}
	if redirectedCalls.Load() != 0 {
		t.Fatalf("redirected endpoint calls = %d, want 0", redirectedCalls.Load())
	}
}

func TestClientValidatesResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "content type", contentType: "text/plain", body: `{}`},
		{name: "malformed JSON", contentType: "application/json", body: `{`},
		{name: "missing metrics", contentType: "application/json", body: `{"messages":[]}`},
		{name: "negative metrics", contentType: "application/json", body: `{"messages":[],"tokens_before":10,"tokens_after":5,"tokens_saved":-1,"compression_ratio":0.5}`},
		{name: "expansion", contentType: "application/json", body: `{"messages":[],"tokens_before":5,"tokens_after":10,"tokens_saved":0,"compression_ratio":1}`},
		{name: "inconsistent metrics", contentType: "application/json", body: `{"messages":[],"tokens_before":10,"tokens_after":5,"tokens_saved":4,"compression_ratio":0.5}`},
		{name: "invalid messages", contentType: "application/json", body: `{"messages":{},"tokens_before":10,"tokens_after":5,"tokens_saved":5,"compression_ratio":0.5}`},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", testCase.contentType)
				_, _ = io.WriteString(w, testCase.body)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Compress(context.Background(), validRequest()); err == nil {
				t.Fatal("Compress succeeded, want validation error")
			}
		})
	}
}

func TestClientRejectsInvalidRequestBeforeCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	invalidRatio := 1.5
	negativeRecent := -1
	if _, err := client.Compress(context.Background(), CompressionRequest{
		Model:    "gpt-4o",
		Messages: json.RawMessage(`{"not":"an array"}`),
	}); err == nil {
		t.Fatal("Compress succeeded, want validation error")
	}
	if _, err := NewClient(server.URL+"/v1/compress", ClientOptions{
		HTTPClient:  server.Client(),
		Compression: CompressionConfig{TargetRatio: &invalidRatio},
	}); err == nil {
		t.Fatal("NewClient accepted invalid target ratio")
	}
	if _, err := NewClient(server.URL+"/v1/compress", ClientOptions{
		HTTPClient:  server.Client(),
		Compression: CompressionConfig{ProtectRecent: &negativeRecent},
	}); err == nil {
		t.Fatal("NewClient accepted negative protect_recent")
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
}

func TestClientHonorsCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/v1/compress", ClientOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Compress(ctx, validRequest())
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Headroom request did not start")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Compress did not return after cancellation")
	}
}

func TestLiveHeadroomContract(t *testing.T) {
	endpoint := os.Getenv("HEADROOM_POC_URL")
	if endpoint == "" {
		t.Skip("set HEADROOM_POC_URL to run against a live Headroom service")
	}

	client, err := NewClient(endpoint, ClientOptions{HTTPClient: &http.Client{Timeout: 30 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	syntheticLog := strings.Repeat(
		"2026-01-01T00:00:00Z INFO worker=synthetic status=ok latency_ms=12\n",
		1000,
	)
	messages, err := json.Marshal([]map[string]any{
		{
			"role":    "user",
			"content": syntheticLog,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defaultResult, err := client.Compress(context.Background(), CompressionRequest{
		Model:    "gpt-4o",
		Messages: messages,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(defaultResult.Messages) == 0 {
		t.Fatal("live Headroom response contained no messages")
	}
	t.Logf("Headroom default: before=%d after=%d saved=%d ratio=%.3f transforms=%v",
		defaultResult.OriginalTokens,
		defaultResult.CompressedTokens,
		defaultResult.TokensSaved,
		defaultResult.CompressionRatio,
		defaultResult.Transforms,
	)

	protectRecent := 0
	targetRatio := 0.5
	resultClient, err := NewClient(endpoint, ClientOptions{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Compression: CompressionConfig{
			CompressUserMessages: true,
			TargetRatio:          &targetRatio,
			ProtectRecent:        &protectRecent,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := resultClient.Compress(context.Background(), CompressionRequest{
		Model:    "gpt-4o",
		Messages: messages,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Headroom explicit compression: before=%d after=%d saved=%d ratio=%.3f transforms=%v",
		result.OriginalTokens,
		result.CompressedTokens,
		result.TokensSaved,
		result.CompressionRatio,
		result.Transforms,
	)
	if result.TokensSaved == 0 {
		t.Fatal("live Headroom request did not exercise a compression transform")
	}
}

func validRequest() CompressionRequest {
	return CompressionRequest{
		Model:    "gpt-4o",
		Messages: json.RawMessage(`[{"role":"user","content":"synthetic content"}]`),
	}
}
