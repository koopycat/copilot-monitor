package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"copilot-monitoring/internal/store"
)

type Handler struct {
	log     io.Writer
	client  *http.Client
	store   *store.Store
	project string
	nextID  atomic.Uint64
}

func NewHandler(log io.Writer) *Handler {
	return NewHandlerWithStore(log, nil, "")
}

func NewHandlerWithStore(log io.Writer, st *store.Store, project string) *Handler {
	return &Handler{
		log:     log,
		store:   st,
		project: project,
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
		fmt.Fprintf(h.log, "request id=%d ts=%s method=%s path=%q route=unknown status=502\n", id, started.Format(time.RFC3339Nano), r.Method, r.URL.RequestURI())
		http.Error(w, "unknown Copilot path", http.StatusBadGateway)
		return
	}

	if route.Local && route.Endpoint == EndpointPing {
		fmt.Fprintf(h.log, "request id=%d ts=%s method=%s path=%q endpoint=%s status=200\n", id, started.Format(time.RFC3339Nano), r.Method, r.URL.RequestURI(), route.Endpoint)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	body, err := readAndRestoreBody(r)
	if err != nil {
		fmt.Fprintf(h.log, "request id=%d read_body_error=%q status=400\n", id, err.Error())
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	meta := ParseRequestMetadata(body)

	fmt.Fprintf(
		h.log,
		"request id=%d ts=%s method=%s path=%q endpoint=%s upstream=%s capture=%s auth_present=%t model=%q stream=%s\n",
		id,
		started.Format(time.RFC3339Nano),
		r.Method,
		r.URL.RequestURI(),
		route.Endpoint,
		route.Upstream,
		route.Capture,
		r.Header.Get("Authorization") != "",
		meta.Model,
		streamLogValue(meta),
	)

	if isWebSocketUpgrade(r) {
		if err := h.proxyWebSocket(id, w, r, route, body); err != nil {
			fmt.Fprintf(h.log, "request id=%d websocket_error=%q\n", id, err.Error())
		}
		return
	}

	outReq, err := MakeUpstreamRequest(r, route, body)
	if err != nil {
		fmt.Fprintf(h.log, "request id=%d build_upstream_error=%q status=502\n", id, err.Error())
		http.Error(w, "failed to build upstream request", http.StatusBadGateway)
		return
	}

	resp, err := h.client.Do(outReq)
	if err != nil {
		if r.Context().Err() != nil {
			fmt.Fprintf(h.log, "request id=%d client_disconnected=true error=%q\n", id, err.Error())
			return
		}
		fmt.Fprintf(h.log, "request id=%d upstream_error=%q status=502\n", id, err.Error())
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
	bytesWritten, err := streamResponse(w, resp.Body, observer)
	if err != nil {
		if r.Context().Err() != nil {
			fmt.Fprintf(h.log, "response id=%d client_disconnected=true bytes=%d error=%q\n", id, bytesWritten, err.Error())
			return
		}
		fmt.Fprintf(h.log, "response id=%d stream_error=%q bytes=%d\n", id, err.Error(), bytesWritten)
		return
	}
	latencyMS := time.Since(started).Milliseconds()
	if observer != nil {
		fmt.Fprintf(
			h.log,
			"response id=%d status=%d bytes=%d latency_ms=%d usage_detected=%t prompt_tokens=%d completion_tokens=%d total_tokens=%d response_model=%q parse_errors=%d\n",
			id,
			resp.StatusCode,
			bytesWritten,
			latencyMS,
			observer.UsageSeen,
			observer.Usage.PromptTokens,
			observer.Usage.CompletionTokens,
			observer.Usage.TotalTokens,
			observer.Model,
			observer.ParseErrors,
		)
		h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, observer, "")
		return
	}
	fmt.Fprintf(h.log, "response id=%d status=%d bytes=%d latency_ms=%d\n", id, resp.StatusCode, bytesWritten, latencyMS)
	h.persistRequest(r.Context(), started, route, r, meta, resp.StatusCode, latencyMS, nil, "")
}

func (h *Handler) persistRequest(ctx context.Context, ts time.Time, route Route, r *http.Request, meta RequestMetadata, status int, latencyMS int64, observer *SSEObserver, errText string) {
	if h.store == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
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
		Timestamp:        ts,
		Endpoint:         string(route.Endpoint),
		Method:           r.Method,
		Path:             r.URL.RequestURI(),
		UpstreamHost:     route.Upstream,
		Model:            model,
		Stream:           meta.Stream,
		Status:           status,
		Error:            errText,
		LatencyMS:        latencyMS,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Project:          h.project,
		RequestHash:      meta.RequestHash,
	}); err != nil {
		fmt.Fprintf(h.log, "store_error=%q\n", err.Error())
	}
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

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		r.Body = http.NoBody
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if closeErr := r.Body.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func MakeUpstreamRequest(in *http.Request, route Route, body []byte) (*http.Request, error) {
	if route.Upstream == "" {
		return nil, fmt.Errorf("route has no upstream")
	}
	outURL := &url.URL{
		Scheme:     "https",
		Host:       route.Upstream,
		Path:       in.URL.Path,
		RawPath:    in.URL.RawPath,
		RawQuery:   in.URL.RawQuery,
		ForceQuery: in.URL.ForceQuery,
	}

	out, err := http.NewRequestWithContext(in.Context(), in.Method, outURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	out.Header = StripHopByHopHeaders(in.Header)
	out.Header.Set("Accept-Encoding", "identity")
	out.Host = route.Upstream
	out.ContentLength = int64(len(body))
	if len(body) == 0 {
		out.Body = http.NoBody
	}
	return out, nil
}

func StripHopByHopHeaders(headers http.Header) http.Header {
	connectionTokens := map[string]struct{}{}
	for _, value := range headers.Values("Connection") {
		for _, rawToken := range strings.Split(value, ",") {
			token := strings.ToLower(strings.TrimSpace(rawToken))
			if token != "" {
				connectionTokens[token] = struct{}{}
			}
		}
	}

	out := make(http.Header, len(headers))
	for name, values := range headers {
		lower := strings.ToLower(name)
		if _, ok := hopByHopHeaders[lower]; ok {
			continue
		}
		if _, ok := connectionTokens[lower]; ok {
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}

var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"proxy-connection":    {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

func copyHeaders(dst, src http.Header) {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range src[key] {
			dst.Add(key, value)
		}
	}
}

func streamResponse(w http.ResponseWriter, body io.Reader, observer *SSEObserver) (int64, error) {
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	var total int64
	defer func() {
		if observer != nil {
			observer.Finish()
		}
	}()
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if observer != nil {
				observer.Observe(chunk)
			}
			written, writeErr := w.Write(chunk)
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return total, nil
			}
			return total, readErr
		}
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && headerContainsToken(r.Header.Get("Connection"), "upgrade")
}

func headerContainsToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func (h *Handler) proxyWebSocket(id uint64, w http.ResponseWriter, r *http.Request, route Route, body []byte) error {
	if len(body) != 0 {
		return fmt.Errorf("websocket request unexpectedly had body bytes=%d", len(body))
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijacking not supported", http.StatusInternalServerError)
		return fmt.Errorf("response writer does not support hijacking")
	}

	upstreamConn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", route.Upstream+":443", &tls.Config{ServerName: route.Upstream, MinVersion: tls.VersionTLS12})
	if err != nil {
		http.Error(w, "upstream websocket dial failed", http.StatusBadGateway)
		return err
	}
	defer upstreamConn.Close()

	upstreamReq := r.Clone(r.Context())
	upstreamReq.URL = &url.URL{
		Scheme:     "https",
		Host:       route.Upstream,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		RawQuery:   r.URL.RawQuery,
		ForceQuery: r.URL.ForceQuery,
	}
	upstreamReq.RequestURI = ""
	upstreamReq.Host = route.Upstream
	upstreamReq.Header = cloneHeaders(r.Header)
	upstreamReq.Body = http.NoBody
	upstreamReq.ContentLength = 0

	if err := upstreamReq.Write(upstreamConn); err != nil {
		http.Error(w, "upstream websocket write failed", http.StatusBadGateway)
		return err
	}

	br := bufio.NewReader(upstreamConn)
	upstreamResp, err := http.ReadResponse(br, upstreamReq)
	if err != nil {
		http.Error(w, "upstream websocket response failed", http.StatusBadGateway)
		return err
	}
	defer upstreamResp.Body.Close()

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer clientConn.Close()

	if clientBuf.Reader.Buffered() != 0 {
		fmt.Fprintf(h.log, "websocket id=%d buffered_client_bytes=%d\n", id, clientBuf.Reader.Buffered())
	}

	if err := upstreamResp.Write(clientConn); err != nil {
		return err
	}
	fmt.Fprintf(h.log, "websocket id=%d status=%d content_type=%q\n", id, upstreamResp.StatusCode, upstreamResp.Header.Get("Content-Type"))
	if upstreamResp.StatusCode != http.StatusSwitchingProtocols {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(upstreamConn, clientConn)
		_ = upstreamConn.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, br)
		_ = clientConn.Close()
	}()
	wg.Wait()
	fmt.Fprintf(h.log, "websocket id=%d complete=true\n", id)
	return nil
}

func cloneHeaders(headers http.Header) http.Header {
	out := make(http.Header, len(headers))
	for name, values := range headers {
		out[name] = append([]string(nil), values...)
	}
	return out
}
