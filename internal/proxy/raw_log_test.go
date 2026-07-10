package proxy

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenRawLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw.jsonl")
	rl, err := OpenRawLogger(path)
	if err != nil {
		t.Fatalf("OpenRawLogger: %v", err)
	}
	defer rl.Close()

	record := RawLogRecord{
		RequestID:    1,
		Timestamp:    time.Now().UTC(),
		Method:       "POST",
		Path:         "/chat/completions",
		Provider:     "copilot",
		Endpoint:     "chat",
		Upstream:     "api.github.com",
		Model:        "gpt-4o",
		Stream:       "true",
		Status:       200,
		LatencyMS:    42,
		RouteMatched: true,
	}

	if err := rl.Write(record); err != nil {
		t.Fatalf("Write: %v", err)
	}
	rl.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var decoded RawLogRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RequestID != record.RequestID {
		t.Errorf("RequestID = %d, want %d", decoded.RequestID, record.RequestID)
	}
	if decoded.Method != record.Method {
		t.Errorf("Method = %q, want %q", decoded.Method, record.Method)
	}
}

func TestRawLoggerNilPath(t *testing.T) {
	rl, err := OpenRawLogger("")
	if err != nil {
		t.Fatalf("OpenRawLogger: %v", err)
	}
	if rl != nil {
		t.Errorf("expected nil RawLogger for empty path, got %v", rl)
	}
	// Verify no panics on nil operations
	if err := rl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := rl.Write(RawLogRecord{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestRawLogRecordSerialization(t *testing.T) {
	record := RawLogRecord{
		RequestID:            2,
		Timestamp:            time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		Method:               "POST",
		Path:                 "/v1/chat/completions",
		Provider:             "openai",
		Endpoint:             "chat",
		Upstream:             "api.openai.com",
		Model:                "gpt-4o",
		Stream:               "false",
		RequestBody:          base64.StdEncoding.EncodeToString([]byte(`{"model":"gpt-4o"}`)),
		RequestBodyTruncated: false,
		Status:               200,
		LatencyMS:            100,
		ResponseHeaders: map[string][]string{
			"content-type": {"application/json"},
		},
		RouteMatched:      true,
		CompressionStatus: "applied",
		PolicyAllowed:     true,
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify all expected fields appear in the JSON
	str := string(data)
	fields := []string{
		`"request_id":2`,
		`"method":"POST"`,
		`"path":"/v1/chat/completions"`,
		`"provider":"openai"`,
		`"endpoint":"chat"`,
		`"upstream":"api.openai.com"`,
		`"model":"gpt-4o"`,
		`"stream":"false"`,
		`"request_body":"`,
		`"request_body_truncated":false`,
		`"status":200`,
		`"latency_ms":100`,
		`"content-type"`,
		`"route_matched":true`,
		`"compression_status":"applied"`,
		`"policy_allowed":true`,
	}
	for _, f := range fields {
		if !strings.Contains(str, f) {
			t.Errorf("JSON missing field: %s", f)
		}
	}
}

func TestEncodeRequestBody(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		enc, truncated := encodeRequestBody(nil)
		if enc != "" {
			t.Errorf("expected empty, got %q", enc)
		}
		if truncated {
			t.Errorf("expected truncated=false for empty body")
		}
	})

	t.Run("under_limit", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o"}`)
		enc, truncated := encodeRequestBody(body)
		if truncated {
			t.Errorf("expected truncated=false for body under limit")
		}
		decoded, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			t.Fatalf("DecodeString: %v", err)
		}
		if string(decoded) != string(body) {
			t.Errorf("decoded = %q, want %q", string(decoded), string(body))
		}
	})

	t.Run("over_limit", func(t *testing.T) {
		// Create body larger than 1024 bytes
		body := []byte(strings.Repeat("x", 2048))
		enc, truncated := encodeRequestBody(body)
		if !truncated {
			t.Errorf("expected truncated=true for body over limit")
		}
		decoded, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			t.Fatalf("DecodeString: %v", err)
		}
		if len(decoded) != 1024 {
			t.Errorf("decoded len = %d, want 1024", len(decoded))
		}
	})

	t.Run("exactly_at_limit", func(t *testing.T) {
		body := []byte(strings.Repeat("a", 1024))
		enc, truncated := encodeRequestBody(body)
		if truncated {
			t.Errorf("expected truncated=false for body exactly at limit")
		}
		decoded, _ := base64.StdEncoding.DecodeString(enc)
		if len(decoded) != 1024 {
			t.Errorf("decoded len = %d, want 1024", len(decoded))
		}
	})
}

func TestRawLoggerConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.jsonl")
	rl, err := OpenRawLogger(path)
	if err != nil {
		t.Fatalf("OpenRawLogger: %v", err)
	}
	defer rl.Close()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			_ = rl.Write(RawLogRecord{RequestID: uint64(n)})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	rl.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
}
