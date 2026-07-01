package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	githubCopilotAPIHost   = "api.githubcopilot.com"
	githubCopilotProxyHost = "copilot-proxy.githubusercontent.com"
)

type config struct {
	addr         string
	maxBodyLog   int
	proxyUnknown bool
	logBodies    bool
}

type upstream struct {
	host string
}

type requestMetadata struct {
	model     string
	stream    bool
	hasStream bool
}

type phase0Handler struct {
	cfg    config
	client *http.Client
	log    *log.Logger
	nextID atomic.Uint64
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.addr, "addr", "127.0.0.1:7733", "HTTP listen address")
	flag.IntVar(&cfg.maxBodyLog, "max-body-log", 2048, "maximum request body bytes to log when --log-bodies is true")
	flag.BoolVar(&cfg.proxyUnknown, "proxy-unknown", false, "forward unknown paths to api.githubcopilot.com instead of returning 502")
	flag.BoolVar(&cfg.logBodies, "log-bodies", false, "log the first N bytes of request bodies")
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	printSettingsSnippet(os.Stdout, cfg.addr)

	h := &phase0Handler{
		cfg: cfg,
		client: &http.Client{Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		}},
		log: logger,
	}

	server := &http.Server{
		Addr:    cfg.addr,
		Handler: h,
	}

	logger.Printf("phase0 listening addr=%s", cfg.addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server failed: %v", err)
	}
}

func printSettingsSnippet(w io.Writer, addr string) {
	baseURL := "http://" + settingsAddr(addr)
	fmt.Fprintf(w, "VSCode settings snippet for %s:\n", baseURL)
	fmt.Fprintf(w, "{\n")
	fmt.Fprintf(w, "  \"github.copilot.advanced\": {\n")
	fmt.Fprintf(w, "    \"debug.overrideProxyUrl\": %q,\n", baseURL)
	fmt.Fprintf(w, "    \"debug.overrideCapiUrl\": %q,\n", baseURL)
	fmt.Fprintf(w, "    \"authProvider\": \"github\"\n")
	fmt.Fprintf(w, "  }\n")
	fmt.Fprintf(w, "}\n\n")
}

func settingsAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func (h *phase0Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := h.nextID.Add(1)
	started := time.Now().UTC()

	body, err := readAndRestoreBody(r)
	if err != nil {
		h.log.Printf("request id=%d read_body_error=%q", id, err.Error())
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	meta := parseRequestMetadata(body)
	h.logRequest(id, started, r, body, meta)

	if r.URL.Path == "/_ping" {
		h.log.Printf("request id=%d route=local_ping action=ok status=200", id)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	up, ok := selectUpstream(r.URL.Path, h.cfg.proxyUnknown)
	if !ok {
		h.log.Printf("request id=%d route=unknown action=reject status=502", id)
		http.Error(w, "unknown path for phase0; use --proxy-unknown to forward unknown paths", http.StatusBadGateway)
		return
	}

	h.log.Printf("request id=%d upstream=%s", id, up.host)
	if isWebSocketUpgrade(r) {
		if err := h.proxyWebSocket(id, w, r, up, body); err != nil {
			h.log.Printf("request id=%d websocket_error=%q", id, err.Error())
		}
		return
	}

	outReq, err := makeUpstreamRequest(r, up, body)
	if err != nil {
		h.log.Printf("request id=%d build_upstream_error=%q", id, err.Error())
		http.Error(w, "failed to build upstream request", http.StatusBadGateway)
		return
	}

	resp, err := h.client.Do(outReq)
	if err != nil {
		if r.Context().Err() != nil {
			h.log.Printf("request id=%d client_disconnected=true error=%q", id, err.Error())
			return
		}
		h.log.Printf("request id=%d upstream_error=%q", id, err.Error())
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), stripHopByHopHeaders(resp.Header))
	w.WriteHeader(resp.StatusCode)

	observer := newSSEObserver(id, h.log)
	h.log.Printf("response id=%d status=%d content_type=%q", id, resp.StatusCode, resp.Header.Get("Content-Type"))
	if err := streamResponse(w, resp.Body, observer); err != nil {
		if r.Context().Err() != nil {
			h.log.Printf("response id=%d client_disconnected=true bytes=%d error=%q", id, observer.bytes, err.Error())
			return
		}
		h.log.Printf("response id=%d stream_error=%q bytes=%d", id, err.Error(), observer.bytes)
		return
	}
	h.log.Printf("response id=%d complete=true bytes=%d usage_detected=%t model=%q", id, observer.bytes, observer.usageSeen, observer.model)
}

func (h *phase0Handler) logRequest(id uint64, ts time.Time, r *http.Request, body []byte, meta requestMetadata) {
	headersJSON, err := json.Marshal(redactHeaders(r.Header))
	if err != nil {
		headersJSON = []byte(`{"error":"failed to marshal headers"}`)
	}

	h.log.Printf(
		"request id=%d ts=%s method=%s target=%q content_type=%q content_length=%d auth_present=%t",
		id,
		ts.Format(time.RFC3339Nano),
		r.Method,
		r.URL.RequestURI(),
		r.Header.Get("Content-Type"),
		r.ContentLength,
		r.Header.Get("Authorization") != "",
	)
	h.log.Printf("request id=%d headers=%s", id, headersJSON)

	if h.cfg.logBodies {
		limit := h.cfg.maxBodyLog
		if limit < 0 {
			limit = 0
		}
		prefix := body
		truncated := false
		if len(prefix) > limit {
			prefix = prefix[:limit]
			truncated = true
		}
		h.log.Printf("request id=%d body_prefix_bytes=%d body_truncated=%t body_prefix=%q", id, len(prefix), truncated, string(prefix))
	}

	if meta.model != "" || meta.hasStream {
		h.log.Printf("request id=%d model=%q stream=%s", id, meta.model, streamLogValue(meta))
	}
}

func streamLogValue(meta requestMetadata) string {
	if !meta.hasStream {
		return "unknown"
	}
	if meta.stream {
		return "true"
	}
	return "false"
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

func selectUpstream(path string, proxyUnknown bool) (upstream, bool) {
	switch {
	case path == "/chat/completions":
		return upstream{host: githubCopilotAPIHost}, true
	case path == "/agents" || strings.HasPrefix(path, "/agents/"):
		return upstream{host: githubCopilotAPIHost}, true
	case path == "/models" || path == "/models/session":
		return upstream{host: githubCopilotAPIHost}, true
	case path == "/responses":
		return upstream{host: githubCopilotAPIHost}, true
	case path == "/embeddings":
		return upstream{host: githubCopilotAPIHost}, true
	case strings.HasPrefix(path, "/v1/engines/"):
		return upstream{host: githubCopilotProxyHost}, true
	case path == "/v1/completions":
		return upstream{host: githubCopilotProxyHost}, true
	case proxyUnknown:
		return upstream{host: githubCopilotAPIHost}, true
	default:
		return upstream{}, false
	}
}

func makeUpstreamRequest(in *http.Request, up upstream, body []byte) (*http.Request, error) {
	outURL := &url.URL{
		Scheme:     "https",
		Host:       up.host,
		Path:       in.URL.Path,
		RawPath:    in.URL.RawPath,
		RawQuery:   in.URL.RawQuery,
		ForceQuery: in.URL.ForceQuery,
	}

	out, err := http.NewRequestWithContext(in.Context(), in.Method, outURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	out.Header = stripHopByHopHeaders(in.Header)
	out.Header.Set("Accept-Encoding", "identity")
	out.Host = up.host
	out.ContentLength = int64(len(body))
	if len(body) == 0 {
		out.Body = http.NoBody
	}
	return out, nil
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

func (h *phase0Handler) proxyWebSocket(id uint64, w http.ResponseWriter, r *http.Request, up upstream, body []byte) error {
	if len(body) != 0 {
		return fmt.Errorf("websocket request unexpectedly had body bytes=%d", len(body))
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijacking not supported", http.StatusInternalServerError)
		return fmt.Errorf("response writer does not support hijacking")
	}

	upstreamConn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", up.host+":443", &tls.Config{ServerName: up.host, MinVersion: tls.VersionTLS12})
	if err != nil {
		http.Error(w, "upstream websocket dial failed", http.StatusBadGateway)
		return err
	}
	defer upstreamConn.Close()

	upstreamReq := r.Clone(r.Context())
	upstreamReq.URL = &url.URL{
		Scheme:     "https",
		Host:       up.host,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		RawQuery:   r.URL.RawQuery,
		ForceQuery: r.URL.ForceQuery,
	}
	upstreamReq.RequestURI = ""
	upstreamReq.Host = up.host
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
		h.log.Printf("websocket id=%d buffered_client_bytes=%d", id, clientBuf.Reader.Buffered())
	}

	if err := upstreamResp.Write(clientConn); err != nil {
		return err
	}
	h.log.Printf("websocket id=%d status=%d content_type=%q", id, upstreamResp.StatusCode, upstreamResp.Header.Get("Content-Type"))
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
	h.log.Printf("websocket id=%d complete=true", id)
	return nil
}

func cloneHeaders(headers http.Header) http.Header {
	out := make(http.Header, len(headers))
	for name, values := range headers {
		out[name] = append([]string(nil), values...)
	}
	return out
}

func parseRequestMetadata(body []byte) requestMetadata {
	var meta requestMetadata
	if len(bytes.TrimSpace(body)) == 0 {
		return meta
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return meta
	}

	if model, ok := findStringKey(value, "model"); ok {
		meta.model = model
	}
	if stream, ok := findBoolKey(value, "stream"); ok {
		meta.stream = stream
		meta.hasStream = true
	}
	return meta
}

func redactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for name, values := range headers {
		copied := append([]string(nil), values...)
		if isSensitiveHeader(name) {
			copied = []string{"<redacted>"}
		}
		redacted[name] = copied
	}
	return redacted
}

func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	return lower == "authorization" ||
		lower == "cookie" ||
		lower == "set-cookie" ||
		lower == "x-github-token" ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret")
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

func stripHopByHopHeaders(headers http.Header) http.Header {
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

func streamResponse(w http.ResponseWriter, body io.Reader, observer *sseObserver) error {
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			observer.observe(chunk)
			if _, err := w.Write(chunk); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

type sseObserver struct {
	requestID uint64
	log       *log.Logger
	buf       []byte
	bytes     int64
	usageSeen bool
	model     string
}

func newSSEObserver(requestID uint64, logger *log.Logger) *sseObserver {
	return &sseObserver{requestID: requestID, log: logger}
}

func (o *sseObserver) observe(chunk []byte) {
	o.bytes += int64(len(chunk))
	o.buf = append(o.buf, chunk...)

	for {
		idx := bytes.IndexByte(o.buf, '\n')
		if idx < 0 {
			if len(o.buf) > 1024*1024 {
				o.buf = o.buf[:0]
			}
			return
		}

		line := append([]byte(nil), o.buf[:idx]...)
		o.buf = o.buf[idx+1:]
		o.processLine(line)
	}
}

func (o *sseObserver) processLine(line []byte) {
	line = bytes.TrimSuffix(line, []byte("\r"))
	trimmed := strings.TrimSpace(string(line))
	if !strings.HasPrefix(trimmed, "data:") {
		return
	}

	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}

	var value any
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return
	}

	if !o.usageSeen {
		if hasKey(value, "usage") {
			o.usageSeen = true
			o.log.Printf("response id=%d usage_detected=true", o.requestID)
		}
	}

	if model, ok := findStringKey(value, "model"); ok && model != "" && model != o.model {
		o.model = model
		o.log.Printf("response id=%d model=%q", o.requestID, model)
	}
}

func hasKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for k, v := range typed {
			if k == key {
				return true
			}
			if hasKey(v, key) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if hasKey(item, key) {
				return true
			}
		}
	}
	return false
}

func findStringKey(value any, key string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed[key]; ok {
			if s, ok := raw.(string); ok {
				return s, true
			}
		}
		for _, v := range typed {
			if s, ok := findStringKey(v, key); ok {
				return s, true
			}
		}
	case []any:
		for _, item := range typed {
			if s, ok := findStringKey(item, key); ok {
				return s, true
			}
		}
	}
	return "", false
}

func findBoolKey(value any, key string) (bool, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed[key]; ok {
			if b, ok := raw.(bool); ok {
				return b, true
			}
		}
		for _, v := range typed {
			if b, ok := findBoolKey(v, key); ok {
				return b, true
			}
		}
	case []any:
		for _, item := range typed {
			if b, ok := findBoolKey(item, key); ok {
				return b, true
			}
		}
	}
	return false, false
}
