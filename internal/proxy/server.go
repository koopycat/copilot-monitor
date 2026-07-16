package proxy

import (
	"bytes"
	"context"
	"encoding/json"
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
	cat             catalog.Catalog
	upstream        string
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
	return &Handler{
		log:        log,
		store:      st,
		project:    project,
		usageDebug: usageDebug,
		startTime:  time.Now(),
		client: &http.Client{Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		}},
	}
}

// SetUpstream sets the single upstream host to forward requests to.
func (h *Handler) SetUpstream(upstream string) {
	h.upstream = upstream
}

// SetCatalog sets the pricing catalog used for per-request cost estimation.
func (h *Handler) SetCatalog(cat catalog.Catalog) {
	h.cat = cat
}

// SetRawLogger sets the raw debug logger on the handler. It must be called
// before the handler is serving requests.
func (h *Handler) SetRawLogger(rl *RawLogger) {
	h.rawLogger = rl
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

	// Built-in ping endpoint
	if r.URL.Path == "/_ping" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// Policy enforcement.
	activePolicy, policyAvailable := h.activePolicy(r.Context(), id)
	if policyAvailable && !activePolicy.Allowed(meta.Model) {
		h.log.Warn("id=%d policy_blocked model=%q\n", id, meta.Model)
		h.persistBlockedRequest(r.Context(), started, r, meta)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		h.writeRawLog(started, id, r, meta, body, nil, false)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "model_blocked",
			"model":   meta.Model,
			"message": "Model is blocked by policy",
		})
		return
	}

	h.log.Request("id=%d method=%s path=%q upstream=%s model=%q stream=%s\n",
		id,
		r.Method,
		r.URL.RequestURI(),
		h.upstream,
		meta.Model,
		streamLogValue(meta),
	)

	if isWebSocketUpgrade(r) {
		if err := h.proxyWebSocket(id, w, r, body); err != nil {
			h.log.Error("id=%d websocket_error=%q\n", id, err.Error())
		}
		return
	}

	outReq, err := MakeUpstreamRequest(r, h.upstream, body)
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

	responseBody := io.Reader(resp.Body)
	if policyAvailable && isModelDiscoveryResponse(r, resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Encoding")) {
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

	observer := newResponseObserver(resp.Header.Get("Content-Type"))
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

	h.persistRequest(r.Context(), started, r, meta, resp.StatusCode, latencyMS, observer)
	h.writeUsageDebug(started, id, r, meta, resp, observer)
	h.writeRawLog(started, id, r, meta, body, resp, true)

	// Record SSE parse errors as anomalies
	if observer != nil && observer.ParseErrors > 0 {
		h.recordAnomaly(store.AnomalyRecord{
			Timestamp: started,
			Category:  "parse_error",
			Severity:  "warn",
			RequestID: id,
			Path:      r.URL.Path,
			Method:    r.Method,
			Model:     meta.Model,
			Detail:    fmt.Sprintf("SSE parse errors: %d", observer.ParseErrors),
		})
	}

	// Emit structured log line
	entry := log.RequestLog{
		RequestID:      id,
		Method:         r.Method,
		Path:           r.URL.RequestURI(),
		Upstream:       h.upstream,
		Model:          meta.Model,
		Status:         resp.StatusCode,
		LatencyMS:      latencyMS,
		CaptureMode:    "usage",
		TokensCaptured: usageSeen,
		UsageMissing:   !usageSeen,
		Endpoint:       r.URL.Path,
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
		entry.CostUSD = costcalc.CostForUsage(
			observer.Usage.PromptTokens,
			observer.Usage.CachedInputTokens,
			observer.Usage.CacheWriteTokens,
			observer.Usage.CompletionTokens,
			lookup.Pricing,
			false,
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

func newResponseObserver(contentType string) *SSEObserver {
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

func (h *Handler) persistBlockedRequest(ctx context.Context, ts time.Time, r *http.Request, meta RequestMetadata) {
	if h.store == nil {
		return
	}
	if err := h.store.InsertRequest(ctx, store.RequestRecord{
		Timestamp:    ts,
		Endpoint:     r.URL.Path,
		Method:       r.Method,
		Path:         r.URL.RequestURI(),
		UpstreamHost: h.upstream,
		Model:        meta.Model,
		Stream:       meta.Stream,
		Status:       403,
		LatencyMS:    0,
		Project:      h.project,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}

func (h *Handler) persistRequest(ctx context.Context, ts time.Time, r *http.Request, meta RequestMetadata, status int, latencyMS int64, observer *SSEObserver) {
	if h.store == nil {
		return
	}
	model := meta.Model
	usage := Usage{}
	usageMissing := observer == nil || !observer.UsageSeen
	if !usageMissing && observer != nil {
		if model == "" && observer.Model != "" {
			model = observer.Model
		}
		usage = observer.Usage
	}
	if err := h.store.InsertRequest(ctx, store.RequestRecord{
		Timestamp:         ts,
		Endpoint:          r.URL.Path,
		Method:            r.Method,
		Path:              r.URL.RequestURI(),
		UpstreamHost:      h.upstream,
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
		UsageMissing:      usageMissing,
	}); err != nil {
		h.log.Warn("store_error=%q\n", err.Error())
	}
}

func (h *Handler) writeRawLog(ts time.Time, id uint64, r *http.Request, meta RequestMetadata, body []byte, resp *http.Response, routeMatched bool) {
	if h.rawLogger == nil {
		return
	}
	reqBodyEnc, reqBodyTrunc := encodeRequestBody(body)
	record := RawLogRecord{
		RequestID:            id,
		Timestamp:            ts,
		Method:               r.Method,
		Path:                 r.URL.RequestURI(),
		Model:                meta.Model,
		Stream:               streamLogValue(meta),
		RequestBody:          reqBodyEnc,
		RequestBodyTruncated: reqBodyTrunc,
		RouteMatched:         routeMatched,
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
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return knownContentTypes[strings.ToLower(ct)]
}
