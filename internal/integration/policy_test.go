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

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/proxy"
	"copilot-monitoring/internal/store"
)

// fakeUpstreamJSON is the response body the fake upstream returns for any request.
const fakeUpstreamJSON = `{"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`

// setupPolicyTest creates the full test harness:
//  1. In-memory SQLite store
//  2. Fake upstream TLS server returning usage JSON for any request
//  3. ProxyConfig with one route: /chat/completions → fake upstream, capture=usage
//  4. Router from config
//  5. Handler with log.Disabled(), store, and router
//  6. Internal HTTP client replaced with one that skips TLS verification
//     so it can talk to the test TLS server.
func setupPolicyTest(t *testing.T) (*store.Store, *proxy.Handler, *httptest.Server) {
	t.Helper()

	// 1. Open in-memory store
	st, err := store.Open(t.TempDir() + "/store.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	// 2. Create fake upstream that returns JSON with usage for any request
	fakeUpstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeUpstreamJSON))
	}))
	t.Cleanup(fakeUpstream.Close)

	u, err := url.Parse(fakeUpstream.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", fakeUpstream.URL, err)
	}

	// 3. Create ProxyConfig with one route pointing to the fake upstream
	cfg := &proxy.ProxyConfig{
		Routes: []proxy.RouteConfig{
			{
				Path:         "/chat/completions",
				UpstreamHost: u.Host,
				Capture:      "usage",
			},
		},
	}

	// 4. Create Router from config
	router := proxy.NewRouter(cfg)

	// 5. Create Handler with router
	h := proxy.NewHandlerWithRouter(log.Disabled(), st, "", nil, router)

	// 6. Replace internal HTTP client with one that skips TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	h.SetTestClient(&http.Client{Transport: tr})

	return st, h, fakeUpstream
}

// sendRequest sends a POST request to /chat/completions with the given JSON body
// through the handler and returns the response recorder.
func sendRequest(t *testing.T, h *proxy.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// mustDecodeError decodes the JSON error response from a blocked request.
func mustDecodeError(t *testing.T, body string) map[string]string {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v; body=%q", err, body)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

func TestIntegration_BlocklistBlocksExactMatch(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	resp := mustDecodeError(t, rec.Body.String())
	if resp["error"] != "model_blocked" {
		t.Fatalf("error = %q, want %q", resp["error"], "model_blocked")
	}
	if resp["model"] != "gpt-4o" {
		t.Fatalf("model = %q, want %q", resp["model"], "gpt-4o")
	}

	// Verify blocked request is persisted
	models, err := st.DistinctModels(ctx)
	if err != nil {
		t.Fatalf("DistinctModels: %v", err)
	}
	found := false
	for _, m := range models {
		if m == "gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gpt-4o not found in persisted models: %v", models)
	}
}

func TestIntegration_BlocklistBlocksPrefixMatch(t *testing.T) {
	st, h, _ := setupPolicyTest(t)

	if err := st.SetPolicy(context.Background(), &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-*"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	resp := mustDecodeError(t, rec.Body.String())
	if resp["model"] != "gpt-4o-mini" {
		t.Fatalf("model = %q, want %q", resp["model"], "gpt-4o-mini")
	}
}

func TestIntegration_BlocklistPassesUnlisted(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"claude-3.5-sonnet","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec.Body.String() != fakeUpstreamJSON {
		t.Fatalf("body = %q, want %q", rec.Body.String(), fakeUpstreamJSON)
	}

	// Verify request is persisted
	models, err := st.DistinctModels(ctx)
	if err != nil {
		t.Fatalf("DistinctModels: %v", err)
	}
	found := false
	for _, m := range models {
		if m == "claude-3.5-sonnet" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("claude-3.5-sonnet not found in persisted models: %v", models)
	}
}

func TestIntegration_AllowlistPassesListed(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Allowlist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec.Body.String() != fakeUpstreamJSON {
		t.Fatalf("body = %q, want %q", rec.Body.String(), fakeUpstreamJSON)
	}

	// Verify persistence
	models, err := st.DistinctModels(ctx)
	if err != nil {
		t.Fatalf("DistinctModels: %v", err)
	}
	found := false
	for _, m := range models {
		if m == "gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gpt-4o not found in persisted models: %v", models)
	}
}

func TestIntegration_AllowlistBlocksUnlisted(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Allowlist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"claude-3.5-sonnet","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	resp := mustDecodeError(t, rec.Body.String())
	if resp["model"] != "claude-3.5-sonnet" {
		t.Fatalf("model = %q, want %q", resp["model"], "claude-3.5-sonnet")
	}

	// Verify blocked request is persisted
	models, err := st.DistinctModels(ctx)
	if err != nil {
		t.Fatalf("DistinctModels: %v", err)
	}
	found := false
	for _, m := range models {
		if m == "claude-3.5-sonnet" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("claude-3.5-sonnet not found in persisted models: %v", models)
	}
}

func TestIntegration_AllowAllPasses(t *testing.T) {
	st, h, _ := setupPolicyTest(t)

	ctx := context.Background()
	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.AllowAll,
		Models: []string{},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec.Body.String() != fakeUpstreamJSON {
		t.Fatalf("body = %q, want %q", rec.Body.String(), fakeUpstreamJSON)
	}
}

func TestIntegration_EmptyModelPasses(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	// Request with no model field — should pass (fail-open for empty model)
	rec := sendRequest(t, h, `{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec.Body.String() != fakeUpstreamJSON {
		t.Fatalf("body = %q, want %q", rec.Body.String(), fakeUpstreamJSON)
	}
}

func TestIntegration_PolicyChangeTakesEffect(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	// Step 1: Set blocklist — gpt-4o is blocked
	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("first request (blocklist): status = %d, want %d; body=%q",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}

	// Step 2: Change policy to allow_all
	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.AllowAll,
		Models: []string{},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	// Step 3: Expire the policy cache so the next request re-fetches
	h.ExpirePolicyCache()

	// Step 4: Now gpt-4o should be allowed
	rec = sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("second request (allow_all): status = %d, want %d; body=%q",
			rec.Code, http.StatusOK, rec.Body.String())
	}

	if rec.Body.String() != fakeUpstreamJSON {
		t.Fatalf("body = %q, want %q", rec.Body.String(), fakeUpstreamJSON)
	}
}

func TestIntegration_BlockedRequestPersisted(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	// Query the store's DB directly to verify exact row contents
	db := st.RawDB()
	rows, err := db.QueryContext(ctx, `
		SELECT model, status,
		       COALESCE(prompt_tokens, 0),
		       COALESCE(completion_tokens, 0),
		       COALESCE(total_tokens, 0)
		FROM requests WHERE model = 'gpt-4o'
	`)
	if err != nil {
		t.Fatalf("query requests: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no request record found for gpt-4o")
	}

	var model string
	var status int
	var promptTokens, completionTokens, totalTokens int
	if err := rows.Scan(&model, &status, &promptTokens, &completionTokens, &totalTokens); err != nil {
		t.Fatalf("scan row: %v", err)
	}

	if model != "gpt-4o" {
		t.Fatalf("model = %q, want %q", model, "gpt-4o")
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, http.StatusForbidden)
	}
	if promptTokens != 0 {
		t.Fatalf("prompt_tokens = %d, want 0 (blocked request)", promptTokens)
	}
	if completionTokens != 0 {
		t.Fatalf("completion_tokens = %d, want 0 (blocked request)", completionTokens)
	}
	if totalTokens != 0 {
		t.Fatalf("total_tokens = %d, want 0 (blocked request)", totalTokens)
	}

	if rows.Next() {
		t.Fatal("expected only one row for gpt-4o, found multiple")
	}
}

func TestIntegration_UnblockedRequestPersisted(t *testing.T) {
	st, h, _ := setupPolicyTest(t)
	ctx := context.Background()

	if err := st.SetPolicy(ctx, &policy.Policy{
		Mode:   policy.Blocklist,
		Models: []string{"gpt-4o"},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	rec := sendRequest(t, h, `{"model":"claude-3.5-sonnet","messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify the request was persisted with correct token counts
	db := st.RawDB()
	rows, err := db.QueryContext(ctx, `
		SELECT model, status,
		       COALESCE(prompt_tokens, 0),
		       COALESCE(completion_tokens, 0),
		       COALESCE(total_tokens, 0)
		FROM requests WHERE model = 'claude-3.5-sonnet'
	`)
	if err != nil {
		t.Fatalf("query requests: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no request record found for claude-3.5-sonnet")
	}

	var model string
	var status int
	var promptTokens, completionTokens, totalTokens int
	if err := rows.Scan(&model, &status, &promptTokens, &completionTokens, &totalTokens); err != nil {
		t.Fatalf("scan row: %v", err)
	}

	if model != "claude-3.5-sonnet" {
		t.Fatalf("model = %q, want %q", model, "claude-3.5-sonnet")
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if promptTokens != 100 {
		t.Fatalf("prompt_tokens = %d, want 100", promptTokens)
	}
	if completionTokens != 50 {
		t.Fatalf("completion_tokens = %d, want 50", completionTokens)
	}
	if totalTokens != 150 {
		t.Fatalf("total_tokens = %d, want 150", totalTokens)
	}

	if rows.Next() {
		t.Fatal("expected only one row for claude-3.5-sonnet, found multiple")
	}
}
