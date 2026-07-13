package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"copilot-monitoring/internal/catalog"
	"copilot-monitoring/internal/compression/headroom"
	costcalc "copilot-monitoring/internal/cost"
	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

const policyCacheTTL = 5 * time.Second

type Handler struct {
	log             *log.Writer
	client          *http.Client
	store           *store.Store
	project         string
	usageDebug      *UsageDebugLogger
	rawLogger       *RawLogger
	router          *Router
	cat             catalog.Catalog
	compressorCache map[string]headroom.MessageCompressor
	compressorMu    sync.Mutex
	nextID          atomic.Uint64
	anomalyRecorder *AnomalyRecorder

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

// activePolicy returns the latest cached policy or loads and caches it. A
// failed load without a cached policy reports unavailable so callers can keep
// the documented fail-open behaviour.
func (h *Handler) activePolicy(ctx context.Context, requestID uint64) (*policy.Policy, bool) {
	if h.store == nil {
		return nil, false
	}

	h.policyMu.RLock()
	fresh := time.Now().Before(h.policyUntil)
	cached := h.policyCache
	h.policyMu.RUnlock()
	if fresh && cached != nil {
		return cached, true
	}

	p, err := h.store.GetPolicy(ctx)
	if err != nil {
		h.log.Warn("id=%d policy_load_error=%q\n", requestID, err.Error())
		if cached != nil {
			return cached, true
		}
		return nil, false
	}

	h.policyMu.Lock()
	h.policyCache = p
	h.policyUntil = time.Now().Add(policyCacheTTL)
	h.policyMu.Unlock()
	return p, true
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

// SetCatalog sets the pricing catalog used for per-request cost estimation.
func (h *Handler) SetCatalog(cat catalog.Catalog) {
	h.cat = cat
}

// SetAnomalyRecorder sets the anomaly recorder for detection hooks.
func (h *Handler) SetAnomalyRecorder(r *AnomalyRecorder) {
	h.anomalyRecorder = r
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
		h.writeRawLog(started, id, Route{}, r, meta, body, nil, provider, compressionMeta{}, false)
		h.recordAnomaly(store.AnomalyRecord{
			Timestamp: started,
			Category:  "unrouted_path",
			Severity:  "warn",
			RequestID: id,
			Path:      r.URL.Path,
			Method:    r.Method,
			Detail:    "no route matched for path",
		})
		http.Error(w, "unknown Copilot path", http.StatusBadGateway)
		return
	}

	// Detect missing auth on non-local routes
	if !route.Local && r.Header.Get("Authorization") == "" && r.Header.Get("authorization") == "" {
		h.recordAnomaly(store.AnomalyRecord{
			Timestamp: started,
			Category:  "auth_missing",
			Severity:  "error",
			RequestID: id,
			Path:      r.URL.Path,
			Method:    r.Method,
			Endpoint:  string(route.Endpoint),
			Detail:    "no Authorization header on proxied request",
		})
	}

	if route.Local {
		h.log.Ping("id=%d path=%q endpoint=%s\n", id, r.URL.RequestURI(), route.Endpoint)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// Policy enforcement.
	activePolicy, policyAvailable := h.activePolicy(r.Context(), id)
	if policyAvailable && !activePolicy.Allowed(meta.Model) {
		h.log.Warn("id=%d policy_blocked model=%q\n", id, meta.Model)
		h.persistBlockedRequest(r.Context(), started, route, r, meta)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		h.writeRawLog(started, id, route, r, meta, body, nil, provider, compressionMeta{}, true)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "model_blocked",
			"model":   meta.Model,
			"message": "Model is blocked by policy",
		})
		return
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

	// Detect unknown Content-Type on captured routes
	contentType := resp.Header.Get("Content-Type")
	if route.Capture != CaptureNone && route.Capture != CaptureTunnel {
		if !isKnownContentType(contentType) {
			h.recordAnomaly(store.AnomalyRecord{
				Timestamp: started,
				Category:  "unknown_content_type",
				Severity:  "info",
				RequestID: id,
				Path:      r.URL.Path,
				Method:    r.Method,
				Endpoint:  string(route.Endpoint),
				Upstream:  route.Upstream,
				Detail:    fmt.Sprintf("unexpected Content-Type: %s", contentType),
			})
		}
	}

	responseBody := io.Reader(resp.Body)
	if policyAvailable && isModelDiscoveryResponse(r, resp.StatusCode, contentType, resp.Header.Get("Content-Encoding")) {
		var filtered bool
		responseBody, filtered = filterModelDiscoveryResponse(resp.Body, activePolicy)
		if filtered {
			resp.Header.Del("Transfer-Encoding")
			if buffered, ok := responseBody.(*bytes.Reader); ok {
				resp.Header.Set("Content-Length", strconv.FormatInt(buffered.Size(), 10))
			}
		}
	}

	copyHeaders(w.Header(), StripHopByHopHeaders(resp.Header))
	w.WriteHeader(resp.StatusCode)

	observer := newResponseObserver(route, contentType)
	bytesWritten, err := streamResponse(w, responseBody, observer)
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
	h.writeRawLog(started, id, route, r, meta, body, resp, provider, compMeta, true)

	// Record SSE parse errors as anomalies
	if observer != nil && observer.ParseErrors > 0 {
		h.recordAnomaly(store.AnomalyRecord{
			Timestamp: started,
			Category:  "parse_error",
			Severity:  "warn",
			RequestID: id,
			Path:      r.URL.Path,
			Method:    r.Method,
			Endpoint:  string(route.Endpoint),
			Upstream:  route.Upstream,
			Model:     meta.Model,
			Detail:    fmt.Sprintf("SSE parse errors: %d", observer.ParseErrors),
		})
	}

	// Emit structured log line
	entry := log.RequestLog{
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
		Endpoint:       string(route.Endpoint),
		Provider:       route.Provider,
	}
	if usageSeen && observer != nil {
		entry.PromptTokens = observer.Usage.PromptTokens
		entry.CompletionTokens = observer.Usage.CompletionTokens
		entry.CachedTokens = observer.Usage.CachedInputTokens
		modelForCost := meta.Model
		if modelForCost == "" && observer.Model != "" {
			modelForCost = observer.Model
		}
		lookup := h.cat.Lookup(modelForCost)
		pricing := lookup.Pricing
		if lookup.Fallback && route.Provider != "" {
			if pf, ok := h.cat.ProviderFallbacks[strings.ToLower(route.Provider)]; ok {
				pricing = pf
			}
		}
		entry.CostUSD = costcalc.CostForUsage(
			observer.Usage.PromptTokens,
			observer.Usage.CachedInputTokens,
			observer.Usage.CacheWriteTokens,
			observer.Usage.CompletionTokens,
			pricing,
			route.NotBilled,
		)
	}
	h.log.RequestLogEntry(entry)
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

func (h *Handler) writeRawLog(ts time.Time, id uint64, route Route, r *http.Request, meta RequestMetadata, body []byte, resp *http.Response, provider string, compMeta compressionMeta, routeMatched bool) {
	if h.rawLogger == nil {
		return
	}
	reqBodyEnc, reqBodyTrunc := encodeRequestBody(body)
	record := RawLogRecord{
		RequestID:            id,
		Timestamp:            ts,
		Method:               r.Method,
		Path:                 r.URL.RequestURI(),
		Provider:             provider,
		Endpoint:             string(route.Endpoint),
		Upstream:             route.Upstream,
		Model:                meta.Model,
		Stream:               streamLogValue(meta),
		RequestBody:          reqBodyEnc,
		RequestBodyTruncated: reqBodyTrunc,
		RouteMatched:         routeMatched,
		CompressionStatus:    compMeta.Status,
	}
	if resp != nil {
		record.Status = resp.StatusCode
		record.ResponseHeaders = SafeHeaders(resp.Header)
		record.PolicyAllowed = true
	}
	if err := h.rawLogger.Write(record); err != nil {
		h.log.Warn("raw_log_error=%q\n", err.Error())
	}
}

// recordAnomaly sends an anomaly record to the background recorder if one is configured.
func (h *Handler) recordAnomaly(rec store.AnomalyRecord) {
	if h.anomalyRecorder != nil {
		h.anomalyRecorder.Record(rec)
	}
}

var knownContentTypes = map[string]bool{
	"text/event-stream":        true,
	"application/json":         true,
	"text/plain":               true,
	"text/html":                true,
	"application/octet-stream": true,
}

func isKnownContentType(ct string) bool {
	// Normalize: take part before semicolon
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return knownContentTypes[strings.ToLower(ct)]
}
