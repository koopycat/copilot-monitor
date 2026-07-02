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
	nextID     atomic.Uint64
}

func NewHandler(log *log.Writer) *Handler {
	return NewHandlerWithStore(log, nil, "")
}

func NewHandlerWithStore(log *log.Writer, st *store.Store, project string) *Handler {
	return NewHandlerWithStoreAndUsageDebug(log, st, project, nil)
}

func NewHandlerWithStoreAndUsageDebug(log *log.Writer, st *store.Store, project string, usageDebug *UsageDebugLogger) *Handler {
	return &Handler{
		log:        log,
		store:      st,
		project:    project,
		usageDebug: usageDebug,
		client: &http.Client{Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		}},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := h.nextID.Add(1)
	started := time.Now().UTC()
	route, ok := RoutePath(r.URL.Path)
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

	body, err := readAndRestoreBody(r)
	if err != nil {
		h.log.Error("id=%d read_body_error=%q\n", id, err.Error())
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	meta := ParseRequestMetadata(body)

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
	var observer *SSEObserver
	if shouldObserveResponse(route, resp.Header.Get("Content-Type")) {
		observer = NewSSEObserver()
	}
	var preview *ResponsePreview
	if resp.StatusCode >= 400 {
		preview = NewResponsePreview(2048)
	}
	bytesWritten, err := streamResponse(w, resp.Body, observer, preview)
	if err != nil {
		if r.Context().Err() != nil {
			h.log.Info("id=%d client_disconnected=true bytes=%d\n", id, bytesWritten)
			return
		}
		h.log.Error("id=%d stream_error=%q bytes=%d\n", id, err.Error(), bytesWritten)
		return
	}
	latencyMS := time.Since(started).Milliseconds()
	previewText := ""
	if preview != nil {
		previewText = preview.String()
	}
	if observer != nil {
		h.log.Response("id=%d status=%d latency_ms=%d bytes=%d prompt_tokens=%d cached=%d cache_write=%d completions=%d total=%d model=%q parse_errors=%d\n",
			id,
			resp.StatusCode,
			bytesWritten,
			latencyMS,
			observer.UsageSeen,
			observer.Usage.PromptTokens,
			observer.Usage.CachedInputTokens,
			observer.Usage.CacheWriteTokens,
			observer.Usage.CompletionTokens,
			observer.Usage.TotalTokens,
			observer.Model,
			observer.ParseErrors,
			previewText,
		)
		h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, observer, previewText)
		h.writeUsageDebug(started, id, route, r, meta, resp, observer)
		return
	}
	h.log.Info("id=%d status=%d bytes=%d latency_ms=%d error_preview=%q\n", id, resp.StatusCode, bytesWritten, latencyMS, previewText)
	h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, nil, previewText)
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

func shouldObserveResponse(route Route, contentType string) bool {
	if route.Capture != CaptureUsage && route.Capture != CaptureMetadata {
		return false
	}
	return strings.Contains(strings.ToLower(contentType), "text/event-stream") || strings.Contains(strings.ToLower(contentType), "json")
}

func (h *Handler) persistRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata, status int, latencyMS int64, observer *SSEObserver, errText string) {
	if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
		return
	}
	if errText == "" && route.Capture == CaptureUsage && (observer == nil || !observer.UsageSeen) {
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
		Error:             errText,
		LatencyMS:         latencyMS,
		PromptTokens:      usage.PromptTokens,
		CachedInputTokens: usage.CachedInputTokens,
		CacheWriteTokens:  usage.CacheWriteTokens,
		CompletionTokens:  usage.CompletionTokens,
		TotalTokens:       usage.TotalTokens,
		Project:           h.project,
		RequestHash:       meta.RequestHash,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}
