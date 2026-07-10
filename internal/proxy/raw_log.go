package proxy

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// RawLogger writes per-request JSON records to a file, including truncated
// request body bytes (base64-encoded) for routing and policy debugging.
// It is opt-in via the --raw-log CLI flag and off by default.
type RawLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// RawLogRecord holds the fields written to the raw debug log for one proxied request.
type RawLogRecord struct {
	RequestID            uint64              `json:"request_id"`
	Timestamp            time.Time           `json:"ts"`
	Method               string              `json:"method"`
	Path                 string              `json:"path"`
	Provider             string              `json:"provider"`
	Endpoint             string              `json:"endpoint"`
	Upstream             string              `json:"upstream"`
	Model                string              `json:"model"`
	Stream               string              `json:"stream"`
	RequestBody          string              `json:"request_body"`
	RequestBodyTruncated bool                `json:"request_body_truncated"`
	Status               int                 `json:"status"`
	LatencyMS            int64               `json:"latency_ms"`
	ResponseHeaders      map[string][]string `json:"response_headers,omitempty"`
	RouteMatched         bool                `json:"route_matched"`
	CompressionStatus    string              `json:"compression_status"`
	PolicyAllowed        bool                `json:"policy_allowed"`
}

const rawLogBodyLimit = 1024

// OpenRawLogger opens a file for appending raw debug records. It returns
// (nil, nil) when path is empty. The file is created with 0600 permissions.
func OpenRawLogger(path string) (*RawLogger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &RawLogger{file: file, enc: json.NewEncoder(file)}, nil
}

// Close closes the underlying file. It is a no-op when the RawLogger is nil.
func (l *RawLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Write appends a JSON-encoded record to the log file. It is a no-op when
// the RawLogger is nil. Concurrent writes are serialised via a mutex.
func (l *RawLogger) Write(record RawLogRecord) error {
	if l == nil || l.enc == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enc.Encode(record)
}

// encodeRequestBody truncates body to rawLogBodyLimit bytes and returns
// the base64-encoded prefix plus a truncated flag.
func encodeRequestBody(body []byte) (encoded string, truncated bool) {
	if len(body) == 0 {
		return "", false
	}
	if len(body) > rawLogBodyLimit {
		return base64.StdEncoding.EncodeToString(body[:rawLogBodyLimit]), true
	}
	return base64.StdEncoding.EncodeToString(body), false
}
