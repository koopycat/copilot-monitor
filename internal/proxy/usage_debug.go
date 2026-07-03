package proxy

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type UsageDebugLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

type UsageDebugRecord struct {
	Timestamp       time.Time           `json:"ts"`
	RequestID       uint64              `json:"request_id"`
	Endpoint        string              `json:"endpoint"`
	Path            string              `json:"path"`
	RequestModel    string              `json:"request_model,omitempty"`
	ResponseModel   string              `json:"response_model,omitempty"`
	Status          int                 `json:"status"`
	ContentType     string              `json:"content_type,omitempty"`
	UsageDetected   bool                `json:"usage_detected"`
	ResponseHeaders map[string][]string `json:"response_headers,omitempty"`
}

func OpenUsageDebugLogger(path string) (*UsageDebugLogger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &UsageDebugLogger{file: file, enc: json.NewEncoder(file)}, nil
}

func (l *UsageDebugLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *UsageDebugLogger) Write(record UsageDebugRecord) error {
	if l == nil || l.enc == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enc.Encode(record)
}

func SafeHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string)
	for name, values := range headers {
		if isSensitiveHeader(name) {
			out[name] = []string{"<redacted>"}
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}

func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	return lower == "authorization" ||
		lower == "cookie" ||
		lower == "set-cookie" ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "credential")
}

func (h *Handler) writeUsageDebug(ts time.Time, id uint64, route Route, r *http.Request, meta RequestMetadata, resp *http.Response, observer *SSEObserver) {
	if h.usageDebug == nil || route.Capture == CaptureNone || route.Capture == CaptureLocal || route.Capture == CaptureTunnel {
		return
	}
	record := UsageDebugRecord{
		Timestamp:       ts,
		RequestID:       id,
		Endpoint:        string(route.Endpoint),
		Path:            r.URL.RequestURI(),
		RequestModel:    meta.Model,
		Status:          resp.StatusCode,
		ContentType:     resp.Header.Get("Content-Type"),
		ResponseHeaders: SafeHeaders(resp.Header),
	}
	if observer != nil {
		record.ResponseModel = observer.Model
		record.UsageDetected = observer.UsageSeen
	}
	if err := h.usageDebug.Write(record); err != nil {
		h.log.Warn("usage_debug_error=%q\n", err.Error())
	}
}
