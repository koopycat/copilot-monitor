package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

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

func (h *Handler) proxyWebSocket(id uint64, w http.ResponseWriter, r *http.Request, body []byte, activePolicy *policy.Policy, policyAvailable bool) error {
	if len(body) != 0 {
		return fmt.Errorf("websocket request unexpectedly had body bytes=%d", len(body))
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijacking not supported", http.StatusInternalServerError)
		return fmt.Errorf("response writer does not support hijacking")
	}

	upstreamAddr, serverName := websocketUpstreamTarget(h.upstream)
	upstreamConn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", upstreamAddr, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
	if err != nil {
		http.Error(w, "upstream websocket dial failed", http.StatusBadGateway)
		return err
	}
	defer upstreamConn.Close()

	upstreamReq := r.Clone(r.Context())
	upstreamReq.URL = &url.URL{
		Scheme:     "https",
		Host:       h.upstream,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		RawQuery:   r.URL.RawQuery,
		ForceQuery: r.URL.ForceQuery,
	}
	upstreamReq.RequestURI = ""
	upstreamReq.Host = h.upstream
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
		h.log.Info("id=%d buffered_client_bytes=%d\n", id, clientBuf.Reader.Buffered())
	}

	if err := upstreamResp.Write(clientConn); err != nil {
		return err
	}
	h.log.Websocket("id=%d status=%d content_type=%q\n", id, upstreamResp.StatusCode, upstreamResp.Header.Get("Content-Type"))
	if upstreamResp.StatusCode != http.StatusSwitchingProtocols {
		return nil
	}

	// Write a debug log entry for the WebSocket connection start.
	h.writeUsageDebugWS(id, r, "", false, 0, 0, 0, 0, 0)

	// Create frame inspectors for both directions. The client inspector holds a
	// complete text message until its model can be checked, while the upstream
	// inspector continues to capture model and usage metadata.
	inspector := &wsInspector{
		h:       h,
		idBase:  id,
		r:       r,
		started: time.Now(),
	}
	clientInspector := &wsClientInspector{
		h:               h,
		idBase:          id,
		r:               r,
		activePolicy:    activePolicy,
		policyAvailable: policyAvailable,
	}
	clientWriter := &wsLockedWriter{w: clientConn}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		result := clientInspector.copyInspected(upstreamConn, clientBuf.Reader)
		if result.Blocked {
			_ = writeWSModelBlockedClose(clientWriter)
		}
		_ = upstreamConn.Close()
	}()
	go func() {
		defer wg.Done()
		inspector.copyInspected(clientWriter, br)
		_ = clientConn.Close()
	}()
	wg.Wait()
	h.log.Websocket("id=%d complete=true\n", id)
	return nil
}

func websocketUpstreamTarget(upstream string) (address, serverName string) {
	if host, _, err := net.SplitHostPort(upstream); err == nil {
		return upstream, host
	}
	return net.JoinHostPort(upstream, "443"), upstream
}

// wsInspector reads WebSocket frames from upstream and forwards them to the
// client while inspecting text frames for model and usage data.
type wsInspector struct {
	h       *Handler
	idBase  uint64
	r       *http.Request
	model   string    // tracked from response.create events
	started time.Time // connection start time for latency
}

// wsClientInspector holds complete client text messages long enough to enforce
// the model policy before the message is sent upstream. It intentionally keeps
// the same fail-open boundary as HTTP requests without a usable model.
type wsClientInspector struct {
	h               *Handler
	idBase          uint64
	r               *http.Request
	activePolicy    *policy.Policy
	policyAvailable bool
}

type wsRelayResult struct {
	Blocked bool
	Stopped bool
}

// wsLockedWriter serializes writes to the client connection. Upstream traffic
// and a policy close frame can otherwise be written concurrently.
type wsLockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *wsLockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (w *wsLockedWriter) writeFrame(frame wsFrame) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeWSFrameDirect(w.w, frame)
}

// copyInspected reads WebSocket frames from src and writes them to dst,
// inspecting text frame payloads for model and usage data.
// Every frame is forwarded as-is (preserving opcode and fin).
// Text frames are reassembled in a separate buffer for JSON inspection
// without affecting what the client receives.
func (w *wsInspector) copyInspected(dst io.Writer, src io.Reader) {
	var inspectBuf []byte
	for {
		frame, err := readWSFrame(src)
		if err != nil {
			if err != io.EOF {
				w.h.log.Warn("id=%d ws_frame_read_error=%q\n", w.idBase, err.Error())
			}
			return
		}

		// Forward every frame as-is to the client.
		if err := writeWSFrame(dst, frame); err != nil {
			return
		}

		// Reassemble fragmented text frames for inspection only.
		switch frame.opcode {
		case wsTextFrame:
			if len(inspectBuf)+len(frame.payload) <= wsMaxPayloadLen {
				inspectBuf = append(inspectBuf, frame.payload...)
			}
			if frame.fin {
				if len(inspectBuf) > 0 {
					w.inspectTextFrame(inspectBuf)
				}
				inspectBuf = inspectBuf[:0]
			}
		case wsContFrame:
			if len(inspectBuf)+len(frame.payload) <= wsMaxPayloadLen {
				inspectBuf = append(inspectBuf, frame.payload...)
			}
			if frame.fin {
				if len(inspectBuf) > 0 {
					w.inspectTextFrame(inspectBuf)
				}
				inspectBuf = inspectBuf[:0]
			}
		}

		if frame.opcode == wsCloseFrame {
			return
		}
	}
}

// copyInspected forwards client frames while buffering each complete text
// message before it reaches the upstream. Interleaved control frames are held
// with the message so the original frame sequence stays intact.
func (w *wsClientInspector) copyInspected(dst io.Writer, src io.Reader) wsRelayResult {
	var textFrames []wsFrame
	var textPayload []byte
	textTooLarge := false

	flushText := func() wsRelayResult {
		if len(textFrames) == 0 {
			return wsRelayResult{}
		}
		result := w.forwardTextMessage(dst, textFrames, textPayload, textTooLarge)
		textFrames = nil
		textPayload = nil
		textTooLarge = false
		return result
	}
	flushUninspected := func() wsRelayResult {
		for _, pending := range textFrames {
			if err := writeWSFrame(dst, pending); err != nil {
				return wsRelayResult{Stopped: true}
			}
		}
		textFrames = nil
		textPayload = nil
		textTooLarge = false
		return wsRelayResult{}
	}

	for {
		frame, err := readWSFrame(src)
		if err != nil {
			if err != io.EOF {
				w.h.log.Warn("id=%d ws_client_frame_read_error=%q\n", w.idBase, err.Error())
			}
			return wsRelayResult{}
		}

		switch frame.opcode {
		case wsTextFrame:
			// A new data frame before the previous fragmented message ended is a
			// protocol violation. Preserve existing behaviour by forwarding the
			// pending message without inspecting it, then begin a new one.
			if len(textFrames) > 0 {
				if result := flushUninspected(); result.Stopped {
					return result
				}
			}
			textFrames = append(textFrames, frame)
			textPayload, textTooLarge = appendWSInspectionPayload(textPayload, frame.payload, textTooLarge)
			if frame.fin {
				if result := flushText(); result.Blocked || result.Stopped {
					return result
				}
			}
		case wsContFrame:
			if len(textFrames) == 0 {
				if err := writeWSFrame(dst, frame); err != nil {
					return wsRelayResult{Stopped: true}
				}
				continue
			}
			textFrames = append(textFrames, frame)
			textPayload, textTooLarge = appendWSInspectionPayload(textPayload, frame.payload, textTooLarge)
			if frame.fin {
				if result := flushText(); result.Blocked || result.Stopped {
					return result
				}
			}
		case wsPingFrame, wsPongFrame:
			if len(textFrames) > 0 {
				textFrames = append(textFrames, frame)
				continue
			}
			if err := writeWSFrame(dst, frame); err != nil {
				return wsRelayResult{Stopped: true}
			}
		case wsCloseFrame:
			if len(textFrames) > 0 {
				if result := flushUninspected(); result.Stopped {
					return result
				}
			}
			if err := writeWSFrame(dst, frame); err != nil {
				return wsRelayResult{Stopped: true}
			}
			return wsRelayResult{}
		default:
			if len(textFrames) > 0 {
				if result := flushUninspected(); result.Stopped {
					return result
				}
			}
			if err := writeWSFrame(dst, frame); err != nil {
				return wsRelayResult{Stopped: true}
			}
		}
	}
}

func appendWSInspectionPayload(existing, payload []byte, tooLarge bool) ([]byte, bool) {
	if tooLarge {
		return nil, true
	}
	if len(existing)+len(payload) > wsMaxPayloadLen {
		return nil, true
	}
	return append(existing, payload...), false
}

func (w *wsClientInspector) forwardTextMessage(dst io.Writer, frames []wsFrame, payload []byte, tooLarge bool) wsRelayResult {
	if tooLarge {
		w.h.log.Warn("id=%d ws_policy_inspection_skipped=payload_too_large\n", w.idBase)
	} else {
		meta := ParseRequestMetadata(payload)
		if w.policyAvailable && w.activePolicy != nil && meta.Model != "" && !w.activePolicy.Allowed(meta.Model) {
			meta.Stream = true
			meta.HasStream = true
			w.h.log.Warn("id=%d ws_policy_blocked model=%q\n", w.idBase, meta.Model)
			w.h.persistBlockedRequest(context.Background(), time.Now().UTC(), w.r, meta)
			return wsRelayResult{Blocked: true}
		}
	}
	for _, frame := range frames {
		if err := writeWSFrame(dst, frame); err != nil {
			return wsRelayResult{Stopped: true}
		}
	}
	return wsRelayResult{}
}

// inspectTextFrame parses a text frame payload as JSON and looks for
// model and usage data from upstream WebSocket events.
func (w *wsInspector) inspectTextFrame(payload []byte) {
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}

	msgType, _ := msg["type"].(string)

	// Record anomaly for unrecognized WebSocket event types
	if msgType != "" && !isKnownWSEvent(msgType) {
		w.h.recordAnomaly(store.AnomalyRecord{
			Timestamp: w.started,
			RequestID: w.idBase,
			Category:  "unknown_ws_event",
			Severity:  "info",
			Path:      w.r.URL.Path,
			Method:    w.r.Method,
			Detail:    fmt.Sprintf("unknown WebSocket event type: %s", msgType),
		})
	}

	// Track model from response.create events.
	if msgType == "response.create" {
		if resp, ok := msg["response"].(map[string]any); ok {
			if model, ok := resp["model"].(string); ok && model != "" {
				w.model = model
			}
		}
	}

	// Extract usage from response.completed events.
	if msgType == "response.completed" {
		resp, _ := msg["response"].(map[string]any)

		// Model: prefer from response.completed, fall back to tracked create model.
		model := w.model
		if resp != nil {
			if m, ok := resp["model"].(string); ok && m != "" {
				model = m
			}
		}

		// Usage: use existing findUsage and findStringKey.
		usage, usageSeen := findUsage(msg)
		if !usageSeen && resp != nil {
			usage, usageSeen = findUsage(resp)
		}

		// Also check for the model from the response wrapper.
		if model == "" && resp != nil {
			if m, ok := findStringKey(resp, "model"); ok {
				model = m
			}
		}

		// Persist and log.
		id := w.h.nextID.Add(1)
		ts := time.Now().UTC()
		latencyMS := ts.Sub(w.started).Milliseconds()

		if w.h.store != nil {
			if err := w.h.store.InsertRequest(context.Background(), storeRequestRecord(
				ts, w.h, w.r, model, true, 200, latencyMS, usage, usageSeen, w.h.project,
			)); err != nil {
				w.h.log.Warn("ws_store_error=%q\n", err.Error())
			}
		}

		w.h.writeUsageDebugWS(id, w.r, model, usageSeen,
			usage.PromptTokens, usage.CachedInputTokens, usage.CacheWriteTokens,
			usage.CompletionTokens, usage.TotalTokens)

		w.h.log.Response("id=%d status=200 latency_ms=%d bytes=0 usage_seen=%t prompt_tokens=%d cached=%d cache_write=%d completions=%d total=%d model=%q parse_errors=0\n",
			id, latencyMS, usageSeen,
			usage.PromptTokens, usage.CachedInputTokens, usage.CacheWriteTokens,
			usage.CompletionTokens, usage.TotalTokens, model)
	}
}

func (h *Handler) writeUsageDebugWS(id uint64, r *http.Request, model string, usageSeen bool, prompt, cached, cacheWrite, completions, total int) {
	if h.usageDebug == nil {
		return
	}
	record := UsageDebugRecord{
		Timestamp:     time.Now().UTC(),
		RequestID:     id,
		Endpoint:      r.URL.Path,
		Path:          r.URL.RequestURI(),
		RequestModel:  model,
		Status:        200,
		UsageDetected: usageSeen,
	}
	_ = h.usageDebug.Write(record)
}

// WebSocket frame opcodes.
const (
	wsContFrame  = 0x0
	wsTextFrame  = 0x1
	wsCloseFrame = 0x8
	wsPingFrame  = 0x9
	wsPongFrame  = 0xA
)

const wsMaxPayloadLen = 1 << 20 // 1 MiB

type wsFrame struct {
	fin     bool
	rsv     byte
	opcode  byte
	masked  bool
	maskKey [4]byte
	payload []byte // decoded for inspection; writer reapplies mask when needed
}

// readWSFrame reads one complete frame, decoding its mask for inspection while
// retaining the frame attributes necessary to relay it unchanged.
func readWSFrame(r io.Reader) (wsFrame, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return wsFrame{}, err
	}
	frame := wsFrame{
		fin:    header[0]&0x80 != 0,
		rsv:    header[0] & 0x70,
		opcode: header[0] & 0x0F,
		masked: header[1]&0x80 != 0,
	}

	payloadLen := uint64(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return wsFrame{}, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return wsFrame{}, err
		}
		payloadLen = binary.BigEndian.Uint64(ext)
	}

	if payloadLen > wsMaxPayloadLen {
		return wsFrame{}, fmt.Errorf("frame payload too large: %d > %d", payloadLen, wsMaxPayloadLen)
	}
	if frame.masked {
		if _, err := io.ReadFull(r, frame.maskKey[:]); err != nil {
			return wsFrame{}, err
		}
	}
	frame.payload = make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, frame.payload); err != nil {
			return wsFrame{}, err
		}
	}
	if frame.masked {
		applyWSMask(frame.payload, frame.maskKey)
	}
	return frame, nil
}

// writeWSFrame writes a frame while preserving FIN, RSV, opcode, and masking.
func writeWSFrame(w io.Writer, frame wsFrame) error {
	if locked, ok := w.(*wsLockedWriter); ok {
		return locked.writeFrame(frame)
	}
	return writeWSFrameDirect(w, frame)
}

func writeWSFrameDirect(w io.Writer, frame wsFrame) error {
	first := frame.rsv | frame.opcode
	if frame.fin {
		first |= 0x80
	}
	header := []byte{first, 0}
	payloadLen := len(frame.payload)
	switch {
	case payloadLen <= 125:
		header[1] = byte(payloadLen)
	case payloadLen <= 65535:
		header[1] = 126
		header = append(header, 0, 0)
		binary.BigEndian.PutUint16(header[2:4], uint16(payloadLen))
	default:
		header[1] = 127
		header = append(header, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[2:10], uint64(payloadLen))
	}
	if frame.masked {
		header[1] |= 0x80
		header = append(header, frame.maskKey[:]...)
	}
	if err := writeWSBytes(w, header); err != nil {
		return err
	}
	payload := frame.payload
	if frame.masked {
		payload = append([]byte(nil), payload...)
		applyWSMask(payload, frame.maskKey)
	}
	return writeWSBytes(w, payload)
}

func writeWSBytes(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func applyWSMask(payload []byte, key [4]byte) {
	for i := range payload {
		payload[i] ^= key[i%len(key)]
	}
}

const (
	wsPolicyViolationCode = 1008
	wsModelBlockedReason  = "model_blocked"
)

func writeWSModelBlockedClose(w io.Writer) error {
	payload := make([]byte, 2+len(wsModelBlockedReason))
	binary.BigEndian.PutUint16(payload[:2], wsPolicyViolationCode)
	copy(payload[2:], wsModelBlockedReason)
	return writeWSFrame(w, wsFrame{fin: true, opcode: wsCloseFrame, payload: payload})
}

// storeRequestRecord builds a store.RequestRecord for persistence.
func storeRequestRecord(ts time.Time, h *Handler, r *http.Request, model string, stream bool, status int, latencyMS int64, usage Usage, usageSeen bool, project string) store.RequestRecord {
	hp := h.isHeadroomProxied(r)
	if !usageSeen {
		return store.RequestRecord{
			Timestamp:       ts,
			Endpoint:        r.URL.Path,
			Method:          r.Method,
			Path:            r.URL.RequestURI(),
			UpstreamHost:    h.upstream,
			Model:           model,
			Stream:          stream,
			Status:          status,
			LatencyMS:       latencyMS,
			Project:         project,
			HeadroomProxied: hp,
		}
	}
	return store.RequestRecord{
		Timestamp:         ts,
		Endpoint:          r.URL.Path,
		Method:            r.Method,
		Path:              r.URL.RequestURI(),
		UpstreamHost:      h.upstream,
		Model:             model,
		Stream:            stream,
		Status:            status,
		LatencyMS:         latencyMS,
		PromptTokens:      usage.PromptTokens,
		CachedInputTokens: usage.CachedInputTokens,
		CacheWriteTokens:  usage.CacheWriteTokens,
		CompletionTokens:  usage.CompletionTokens,
		TotalTokens:       usage.TotalTokens,
		Project:           project,
		HeadroomProxied:   hp,
	}
}

func cloneHeaders(headers http.Header) http.Header {
	out := make(http.Header, len(headers))
	for name, values := range headers {
		out[name] = append([]string(nil), values...)
	}
	return out
}

// knownWSEvents lists Copilot Responses API event types that the proxy expects.
// Any text frame with a type not in this set triggers an anomaly.
var knownWSEvents = map[string]bool{
	"response.create":                          true,
	"response.created":                         true,
	"response.text.delta":                      true,
	"response.text.done":                       true,
	"response.audio.delta":                     true,
	"response.audio.done":                      true,
	"response.code_interpreter.call_started":   true,
	"response.code_interpreter.call.completed": true,
	"response.completed":                       true,
	"response.cancelled":                       true,
	"response.failed":                          true,
	"response.in_progress":                     true,
	"response.output_item.added":               true,
	"response.output_item.done":                true,
	"response.content_part.added":              true,
	"response.content_part.done":               true,
	"rate_limit.updated":                       true,
	"session.created":                          true,
	"session.updated":                          true,
	"error":                                    true,
	"ping":                                     true,
}

func isKnownWSEvent(msgType string) bool {
	return knownWSEvents[msgType]
}
