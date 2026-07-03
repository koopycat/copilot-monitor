package proxy

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageDebugLoggerWritesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	logger, err := OpenUsageDebugLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	err = logger.Write(UsageDebugRecord{
		Timestamp:     time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		RequestID:     1,
		Endpoint:      "chat",
		Path:          "/chat/completions",
		RequestModel:  "gpt-5-mini",
		Status:        200,
		UsageDetected: true,
	})
	if closeErr := logger.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing JSONL row")
	}
	var record UsageDebugRecord
	if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.RequestModel != "gpt-5-mini" || !record.UsageDetected {
		t.Fatalf("record = %#v", record)
	}
}

func TestSafeHeadersRedactsSensitiveValues(t *testing.T) {
	headers := http.Header{
		"Authorization": {"Bearer secret"},
		"Set-Cookie":    {"a=b"},
		"X-Request-Id":  {"safe"},
	}
	got := SafeHeaders(headers)
	if got["Authorization"][0] != "<redacted>" || got["Set-Cookie"][0] != "<redacted>" {
		t.Fatalf("sensitive headers were not redacted: %#v", got)
	}
	if got["X-Request-Id"][0] != "safe" {
		t.Fatalf("safe header changed: %#v", got)
	}
}
