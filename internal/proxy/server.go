package proxy

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/store"
)

type Handler struct {
	log        *log.Writer
	client     *http.Client
	store      *store.Store
	project    string
	usageDebug *UsageDebugLogger
	router     *Router
	nextID     atomic.Uint64
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
		client: &http.Client{Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		}},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := h.nextID.Add(1)
	started := time.Now().UTC()

	// Read body first so model is available for routing
	body, err := readAndRestoreBody(r)
	if err != nil {
		h.log.Error("id=%d read_body_error=%q\n", id, err.Error())
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	meta := ParseRequestMetadata(body)

	// Route by path + model
	route, ok := h.router.MatchModel(r.URL.Path, meta.Model)
	if !ok {
		h.log.Error("id=%d path=%q route=unknown status=502\n", id, r.URL.RequestURI())
		http.Error(w, "unknown Copilot path", http.StatusBadGateway)
		return
	}

	if route.Local && route.Endpoint == EndpointPing {
		h.log.Ping("id=%d path=%q endpoint=%s\n", id, r.URL.RequestURI(), route.Endpoint)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	h.log.Request("id=%d method=%s path=%q endpoint=%s upstream=%s capture=%s model=%q stream=%s\n",
		id,
		r.Method,
		r.URL.RequestURI(),
		route.Endpoint,
		route.Upstream,
		route.Capture,
		meta.Model,
		streamLogValue(meta),
	)

	if isWebSocketUpgrade(r) {
		if err := h.proxyWebSocket(id, w, r, route, body); err != nil {
			h.log.Error("id=%d websocket_error=%q\n", id, err.Error())
		}
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
	if observer != nil {
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
		h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, observer)
		h.writeUsageDebug(started, id, route, r, meta, resp, observer)
		return
	}
	h.log.Info("id=%d status=%d bytes=%d latency_ms=%d\n", id, resp.StatusCode, bytesWritten, latencyMS)
	h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, nil)
	h.writeUsageDebug(started, id, route, r, meta, resp, nil)
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

func (h *Handler) persistRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata, status int, latencyMS int64, observer *SSEObserver) {
	if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
		return
	}
	if route.Capture == CaptureUsage && (observer == nil || !observer.UsageSeen) {
		return
	}
	model := meta.Model
	usage := Usage{}
	if observer != nil {
		// Prefer the request model because Copilot often emits response model names for
		// internal helper calls. The response model is only a fallback when the request
		// body did not expose a model.
		if model == "" && observer.Model != "" {
			model = observer.Model
		}
		usage = observer.Usage
	}
	if err := h.store.InsertRequest(ctx, store.RequestRecord{
		Timestamp:         ts,
		Endpoint:          string(route.Endpoint),
		Method:            r.Method,
		Path:              r.URL.RequestURI(),
		UpstreamHost:      route.Upstream,
		Model:             model,
		Stream:            meta.Stream,
		Status:            status,
		LatencyMS:         latencyMS,
		PromptTokens:      usage.PromptTokens,
		CachedInputTokens: usage.CachedInputTokens,
		CacheWriteTokens:  usage.CacheWriteTokens,
		CompletionTokens:  usage.CompletionTokens,
		TotalTokens:       usage.TotalTokens,
		Project:           h.project,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}
