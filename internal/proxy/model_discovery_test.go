package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

const modelListResponse = `{"object":"list","after":"cursor","data":[{"id":"gpt-4o","object":"model","owned_by":"openai"},{"id":"claude-3-7-sonnet","object":"model","owned_by":"anthropic"},{"id":"gpt-4o-mini","object":"model","owned_by":"openai"}]}`

func newModelDiscoveryHandler(t *testing.T, p *policy.Policy, body string) *Handler {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	require.NoError(t, st.SetPolicy(context.Background(), p))

	h := NewHandlerWithStore(log.Disabled(), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})})
	return h
}

func modelIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var response struct {
		Data []struct {
			ID     string `json:"id"`
			Owner  string `json:"owned_by"`
			Object string `json:"object"`
		} `json:"data"`
		Object string `json:"object"`
		After  string `json:"after"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	assert.Equal(t, "list", response.Object)
	assert.Equal(t, "cursor", response.After)
	for _, model := range response.Data {
		assert.Equal(t, "model", model.Object)
		assert.NotEmpty(t, model.Owner)
	}
	returnValues := make([]string, len(response.Data))
	for i, model := range response.Data {
		returnValues[i] = model.ID
	}
	return returnValues
}

func TestHandlerFiltersModelDiscoveryByPolicy(t *testing.T) {
	tests := []struct {
		name string
		p    *policy.Policy
		want []string
	}{
		{
			name: "allowlist respects exact and wildcard patterns",
			p:    &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o", "claude-*"}},
			want: []string{"gpt-4o", "claude-3-7-sonnet"},
		},
		{
			name: "blocklist omits matching model",
			p:    &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}},
			want: []string{"claude-3-7-sonnet", "gpt-4o-mini"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newModelDiscoveryHandler(t, tt.p, modelListResponse)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/models", nil))

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.want, modelIDs(t, rec.Body.Bytes()))
			assert.Equal(t, int64(rec.Body.Len()), rec.Result().ContentLength)
		})
	}
}

func TestHandlerLeavesModelDiscoveryUnchangedWhenFilteringIsUnavailable(t *testing.T) {
	tests := []struct {
		name string
		p    *policy.Policy
		body string
	}{
		{
			name: "allow all policy",
			p:    &policy.Policy{Mode: policy.AllowAll},
			body: modelListResponse,
		},
		{
			name: "malformed model-list envelope",
			p:    &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o"}},
			body: `{"models":[{"id":"gpt-4o-mini"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newModelDiscoveryHandler(t, tt.p, tt.body)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/models", nil))

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.body, rec.Body.String())
		})
	}
}

func TestFilterModelDiscoveryResponseLeavesUnknownPolicyModeUnchanged(t *testing.T) {
	filtered, changed := filterModelDiscoveryResponse(strings.NewReader(modelListResponse), &policy.Policy{
		Mode:   "unknown",
		Models: []string{"gpt-4o"},
	})
	got, err := io.ReadAll(filtered)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, modelListResponse, string(got))
}

func TestFilterModelDiscoveryResponseLeavesOversizedBodyUnchanged(t *testing.T) {
	body := []byte(`{"data":[{"id":"gpt-4o"}],"padding":"` + strings.Repeat("x", maxModelDiscoveryResponseBytes) + `"}`)

	filtered, changed := filterModelDiscoveryResponse(bytes.NewReader(body), &policy.Policy{
		Mode:   policy.Allowlist,
		Models: []string{"gpt-4o"},
	})
	got, err := io.ReadAll(filtered)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, body, got)
}

func TestHandlerLeavesModelDiscoveryUnchangedWithoutPolicyStore(t *testing.T) {
	h := NewHandler(log.Disabled())
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(modelListResponse)),
		}, nil
	})})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/models", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, modelListResponse, rec.Body.String())
}

func TestHandlerPolicyStillBlocksDirectRequestAfterDiscovery(t *testing.T) {
	h := newModelDiscoveryHandler(t, &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o"}}, modelListResponse)

	discovery := httptest.NewRecorder()
	h.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, "/models", nil))
	assert.Equal(t, []string{"gpt-4o"}, modelIDs(t, discovery.Body.Bytes()))

	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini"}`)))
	assert.Equal(t, http.StatusForbidden, blocked.Code)

	allowed := httptest.NewRecorder()
	h.ServeHTTP(allowed, httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`)))
	assert.Equal(t, http.StatusOK, allowed.Code)
}

type maxReadSizeReader struct {
	reader  io.Reader
	maxSize int
}

func (r *maxReadSizeReader) Read(p []byte) (int, error) {
	if len(p) > r.maxSize {
		return 0, errors.New("response was buffered instead of streamed")
	}
	return r.reader.Read(p)
}

func TestHandlerDoesNotBufferNonDiscoveryResponsesForPolicyFiltering(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "store.db"))
	require.NoError(t, err)
	defer st.Close()
	require.NoError(t, st.SetPolicy(context.Background(), &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o"}}))

	h := NewHandlerWithStore(log.Disabled(), st, "")
	h.SetUpstream("api.githubcopilot.com")
	h.SetTestClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(&maxReadSizeReader{
				reader:  bytes.NewBufferString(`{"choices":[]}`),
				maxSize: 32 << 10,
			}),
		}, nil
	})})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`)))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `{"choices":[]}`, rec.Body.String())
}
