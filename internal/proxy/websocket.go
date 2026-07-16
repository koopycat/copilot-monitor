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

func (h *Handler) proxyWebSocket(id uint64, w http.ResponseWriter, r *http.Request, body []byte) error {
	if len(body) != 0 {
		return fmt.Errorf("websocket request unexpectedly had body bytes=%d", len(body))
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijacking not supported", http.StatusInternalServerError)
		return fmt.Errorf("response writer does not support hijacking")
	}

	upstreamConn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", h.upstream+":443", &tls.Config{ServerName: h.upstream, MinVersion: tls.VersionTLS12})
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

	// Create the frame inspector for upstream->client traffic.
	inspector := &wsInspector{
		h:       h,
		idBase:  id,
		r:       r,
		started: time.Now(),
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
		inspector.copyInspected(clientConn, br)
		_ = clientConn.Close()
	}()
	wg.Wait()
	h.log.Websocket("id=%d complete=true\n", id)
	return nil
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

// copyInspected reads WebSocket frames from src and writes them to dst,
// inspecting text frame payloads for model and usage data.
// Every frame is forwarded as-is (preserving opcode and fin).
// Text frames are reassembled in a separate buffer for JSON inspection
// without affecting what the client receives.
func (w *wsInspector) copyInspected(dst io.Writer, src io.Reader) {
	var inspectBuf []byte
	for {
		payload, opcode, fin, err := readWSFrame(src)
		if err != nil {
			if err != io.EOF {
				w.h.log.Warn("id=%d ws_frame_read_error=%q\n", w.idBase, err.Error())
			}
			return
		}

		// Forward every frame as-is to the client.
		if err := writeWSFrame(dst, opcode, payload); err != nil {
			return
		}

		// Reassemble fragmented text frames for inspection only.
		switch opcode {
		case wsTextFrame:
			if len(inspectBuf)+len(payload) <= wsMaxPayloadLen {
				inspectBuf = append(inspectBuf, payload...)
			}
			if fin {
				if len(inspectBuf) > 0 {
					w.inspectTextFrame(inspectBuf)
				}
				inspectBuf = inspectBuf[:0]
			}
		case wsContFrame:
			if len(inspectBuf)+len(payload) <= wsMaxPayloadLen {
				inspectBuf = append(inspectBuf, payload...)
			}
			if fin {
				if len(inspectBuf) > 0 {
					w.inspectTextFrame(inspectBuf)
				}
				inspectBuf = inspectBuf[:0]
			}
		}

		if opcode == wsCloseFrame {
			return
		}
	}
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

// readWSFrame reads a single WebSocket frame from r.
// Returns payload, opcode, fin flag, and any error.
// For upstream->client frames, masking is not expected (server->client = unmasked).
func readWSFrame(r io.Reader) (payload []byte, opcode byte, fin bool, err error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, 0, false, err
	}
	fin = header[0]&0x80 != 0
	opcode = header[0] & 0x0F

	payloadLen := uint64(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, 0, false, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, 0, false, err
		}
		payloadLen = binary.BigEndian.Uint64(ext)
	}

	if payloadLen > wsMaxPayloadLen {
		return nil, 0, false, fmt.Errorf("frame payload too large: %d > %d", payloadLen, wsMaxPayloadLen)
	}
	payload = make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, 0, false, err
		}
	}
	return payload, opcode, fin, nil
}

// writeWSFrame writes a single WebSocket frame to w.
func writeWSFrame(w io.Writer, opcode byte, payload []byte) error {
	frame := make([]byte, 2, 10+len(payload))
	frame[0] = 0x80 | opcode // FIN=1

	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		frame[1] = byte(payloadLen)
	case payloadLen <= 65535:
		frame[1] = 126
		frame = append(frame, 0, 0)
		binary.BigEndian.PutUint16(frame[2:4], uint16(payloadLen))
	default:
		frame[1] = 127
		frame = append(frame, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(frame[2:10], uint64(payloadLen))
	}
	frame = append(frame, payload...)
	_, err := w.Write(frame)
	return err
}

// storeRequestRecord builds a store.RequestRecord for persistence.
func storeRequestRecord(ts time.Time, h *Handler, r *http.Request, model string, stream bool, status int, latencyMS int64, usage Usage, usageSeen bool, project string) store.RequestRecord {
	if !usageSeen {
		return store.RequestRecord{
			Timestamp:    ts,
			Endpoint:     r.URL.Path,
			Method:       r.Method,
			Path:         r.URL.RequestURI(),
			UpstreamHost: h.upstream,
			Model:        model,
			Stream:       stream,
			Status:       status,
			LatencyMS:    latencyMS,
			Project:      project,
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
