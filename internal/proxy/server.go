package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"llm-proxy/internal/compression/headroom"
	"llm-proxy/internal/log"
	"llm-proxy/internal/policy"
	"llm-proxy/internal/store"
)

const policyCacheTTL = 5 * time.Second

type Handler struct {
	log                 *log.Writer
	client              *http.Client
	store               *store.Store
	project             string
	usageDebug          *UsageDebugLogger
	router              *Router
	compressor          headroom.MessageCompressor
	compressionRequired bool
	nextID              atomic.Uint64

	policyMu     sync.RWMutex
	policyCache  *policy.Policy
	policyUntil  time.Time
	requestCount atomic.Int64
	startTime    time.Time
}

func (h *Handler) RequestCount() int64 {
	return h.requestCount.Load()
}

func (h *Handler) Uptime() time.Duration {
	if h.startTime.IsZero() {
		return 0
	}
	return time.Since(h.startTime)
}

func NewHandler(log *log.Writer) *Handler {
	return NewHandlerWithStore(log, nil, "")
}

func NewHandlerWithStore(log *log.Writer, st *store.Store, project string) *Handler {
	return NewHandlerWithStoreAndUsageDebug(log, st, project, nil)
}

func NewHandlerWithStoreAndUsageDebug(log *log.Writer, st *store.Store, project string, usageDebug *UsageDebugLogger) *Handler {
	return NewHandlerWithRouter(log, st, project, usageDebug, nil)
}

func NewHandlerWithRouter(log *log.Writer, st *store.Store, project string, usageDebug *UsageDebugLogger, router *Router) *Handler {
	if router == nil {
		router = NewRouter(nil)
	}
	return &Handler{
		log:        log,
		store:      st,
		project:    project,
		usageDebug: usageDebug,
		router:     router,
		startTime:  time.Now(),
		client: &http.Client{Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		}},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := h.nextID.Add(1)
	h.requestCount.Add(1)
	started := time.Now().UTC()

	// Read body first so model is available for routing
	body, err := readAndRestoreBody(r)
	if err != nil {
		h.log.Error("id=%d read_body_error=%q\n", id, err.Error())
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	meta := ParseRequestMetadata(body)

	// Strip provider prefix from URL path
	originalURI := r.URL.RequestURI()
	provider, remainingPath := StripProviderPrefix(r.URL.Path)
	if provider != "" {
		r.URL.Path = remainingPath
		if r.URL.RawPath != "" {
			_, remainingRaw := StripProviderPrefix(r.URL.RawPath)
			r.URL.RawPath = remainingRaw
		}
	}

	// Built-in health endpoint
	if r.URL.Path == "/_health" {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"status":         "ok",
			"uptime_seconds": int64(time.Since(h.startTime).Seconds()),
			"requests_total": h.requestCount.Load(),
			"db_size_bytes":  0,
		}
		if h.store != nil {
			if _, err := h.store.DistinctModels(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				resp["status"] = "error"
				resp["error"] = "store unreachable: " + err.Error()
				json.NewEncoder(w).Encode(resp)
				return
			}
			if fi, err := os.Stat(h.store.DBPath()); err == nil {
				resp["db_size_bytes"] = fi.Size()
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Route by path + model + provider
	route, ok := h.router.MatchModel(r.URL.Path, meta.Model, provider)
	if !ok {
		h.log.Error("id=%d path=%q provider=%q route=unknown status=502\n", id, originalURI, provider)
		http.Error(w, "unknown Copilot path", http.StatusBadGateway)
		return
	}

	if route.Local {
		h.log.Ping("id=%d path=%q endpoint=%s\n", id, r.URL.RequestURI(), route.Endpoint)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// Policy enforcement.
	{
		allowed := true
		if h.store != nil {
			h.policyMu.RLock()
			fresh := time.Now().Before(h.policyUntil)
			cached := h.policyCache
			h.policyMu.RUnlock()

			if !fresh {
				p, err := h.store.GetPolicy(r.Context())
				if err != nil {
					h.log.Warn("id=%d policy_load_error=%q\n", id, err.Error())
					if cached != nil {
						allowed = cached.Allowed(meta.Model)
					}
				} else {
					h.policyMu.Lock()
					if time.Now().After(h.policyUntil) {
						h.policyCache = p
						h.policyUntil = time.Now().Add(policyCacheTTL)
					}
					h.policyMu.Unlock()
					allowed = p.Allowed(meta.Model)
				}
			} else if cached != nil {
				allowed = cached.Allowed(meta.Model)
			}
		}

		if !allowed {
			h.log.Warn("id=%d policy_blocked model=%q\n", id, meta.Model)
			h.persistBlockedRequest(r.Context(), started, route, r, meta)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "model_blocked",
				"model":   meta.Model,
				"message": "Model is blocked by policy",
			})
			return
		}
	}

	h.log.Request("id=%d method=%s path=%q endpoint=%s upstream=%s capture=%s provider=%q model=%q stream=%s\n",
		id,
		r.Method,
		originalURI,
		route.Endpoint,
		route.Upstream,
		route.Capture,
		provider,
		meta.Model,
		streamLogValue(meta),
	)

	if isWebSocketUpgrade(r) {
		if err := h.proxyWebSocket(id, w, r, route, body); err != nil {
			h.log.Error("id=%d websocket_error=%q\n", id, err.Error())
		}
		return
	}

	var compMeta compressionMeta
	body, err = h.maybeCompress(r.Context(), id, r, route, body, &compMeta)
	if err != nil {
		if errors.Is(err, errCompressionRequired) {
			http.Error(w, "request compression failed", http.StatusBadGateway)
			return
		}
		h.log.Error("id=%d compression_error=true\n", id)
		http.Error(w, "request compression failed", http.StatusBadGateway)
		return
	}

	outReq, err := MakeUpstreamRequest(r, route, body)
	if err != nil {
		h.log.Error("id=%d build_upstream_error=%q\n", id, err.Error())
		http.Error(w, "failed to build upstream request", http.StatusBadGateway)
		return
	}

	resp, err := h.client.Do(outReq)
	if err != nil {
		if r.Context().Err() != nil {
			h.log.Info("id=%d client_disconnected=true\n", id)
			return
		}
		h.log.Error("id=%d upstream_error=%q\n", id, err.Error())
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), StripHopByHopHeaders(resp.Header))
	w.WriteHeader(resp.StatusCode)
	observer := newResponseObserver(route, resp.Header.Get("Content-Type"))
	bytesWritten, err := streamResponse(w, resp.Body, observer)
	if err != nil {
		if r.Context().Err() != nil {
			h.log.Info("id=%d client_disconnected=true bytes=%d\n", id, bytesWritten)
			return
		}
		h.log.Error("id=%d stream_error=%q bytes=%d\n", id, err.Error(), bytesWritten)
		return
	}
	latencyMS := time.Since(started).Milliseconds()
	usageSeen := observer != nil && observer.UsageSeen

	if usageSeen {
		h.log.Response("id=%d status=%d latency_ms=%d bytes=%d usage_seen=%t prompt_tokens=%d cached=%d cache_write=%d completions=%d total=%d model=%q parse_errors=%d\n",
			id,
			resp.StatusCode,
			latencyMS,
			bytesWritten,
			observer.UsageSeen,
			observer.Usage.PromptTokens,
			observer.Usage.CachedInputTokens,
			observer.Usage.CacheWriteTokens,
			observer.Usage.CompletionTokens,
			observer.Usage.TotalTokens,
			observer.Model,
			observer.ParseErrors,
		)
	} else {
		h.log.Info("id=%d status=%d bytes=%d latency_ms=%d\n", id, resp.StatusCode, bytesWritten, latencyMS)
	}

	h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, compMeta, observer)
	h.writeUsageDebug(started, id, route, r, meta, resp, observer)

	// Emit structured log line
	h.log.RequestLogEntry(log.RequestLog{
		RequestID:      id,
		Method:         r.Method,
		Path:           r.URL.RequestURI(),
		Upstream:       route.Upstream,
		Model:          meta.Model,
		Status:         resp.StatusCode,
		LatencyMS:      latencyMS,
		CaptureMode:    string(route.Capture),
		TokensCaptured: usageSeen,
		UsageMissing:   !usageSeen && route.Capture == CaptureUsage,
	})
}

func streamLogValue(meta RequestMetadata) string {
	if !meta.HasStream {
		return "unknown"
	}
	if meta.Stream {
		return "true"
	}
	return "false"
}

func newResponseObserver(route Route, contentType string) *SSEObserver {
	if route.Capture != CaptureUsage && route.Capture != CaptureMetadata {
		return nil
	}
	contentType = strings.ToLower(contentType)
	switch {
	case strings.Contains(contentType, "text/event-stream"):
		return NewSSEObserver()
	case strings.Contains(contentType, "json"):
		return NewJSONObserver()
	default:
		return nil
	}
}

func (h *Handler) persistBlockedRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata) {
	if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
		return
	}
	if err := h.store.InsertRequest(ctx, store.RequestRecord{
		Timestamp:    ts,
		Endpoint:     string(route.Endpoint),
		Method:       r.Method,
		Path:         r.URL.RequestURI(),
		UpstreamHost: route.Upstream,
		Model:        meta.Model,
		Stream:       meta.Stream,
		Status:       403,
		LatencyMS:    0,
		Project:      h.project,
		NotBilled:    route.NotBilled,
		Provider:     route.Provider,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}

func (h *Handler) persistRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata, status int, latencyMS int64, compMeta compressionMeta, observer *SSEObserver) {
	if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
		return
	}
	model := meta.Model
	usage := Usage{}
	usageMissing := false
	if route.Capture == CaptureUsage && (observer == nil || !observer.UsageSeen) {
		usageMissing = true
	} else if observer != nil {
		// Prefer the request model because upstreams may emit different model names
		// for internal helper calls. The response model is only a fallback when the
		// request body did not expose a model.
		if model == "" && observer.Model != "" {
			model = observer.Model
		}
		usage = observer.Usage
	}
	if err := h.store.InsertRequest(ctx, store.RequestRecord{
		Timestamp:                 ts,
		Endpoint:                  string(route.Endpoint),
		Method:                    r.Method,
		Path:                      r.URL.RequestURI(),
		UpstreamHost:              route.Upstream,
		Model:                     model,
		Stream:                    meta.Stream,
		Status:                    status,
		LatencyMS:                 latencyMS,
		PromptTokens:              usage.PromptTokens,
		CachedInputTokens:         usage.CachedInputTokens,
		CacheWriteTokens:          usage.CacheWriteTokens,
		CompletionTokens:          usage.CompletionTokens,
		TotalTokens:               usage.TotalTokens,
		Project:                   h.project,
		NotBilled:                 route.NotBilled,
		Provider:                  route.Provider,
		UsageMissing:              usageMissing,
		CompressionStatus:         compMeta.Status,
		CompressionOriginalTokens: compMeta.OriginalTokens,
		CompressionFinalTokens:    compMeta.FinalTokens,
		CompressionLatencyMS:      compMeta.LatencyMS,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}
