package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriterNoColorsForBuffer(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)
	w.Request("id=%d method=%s\n", 1, "GET")
	got := buf.String()
	if got != "→ id=1 method=GET\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriterJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatJSON)
	w.RequestLogEntry(RequestLog{
		RequestID:      1,
		Method:         "POST",
		Path:           "/chat",
		Upstream:       "api.example.com",
		Model:          "gpt-4o",
		Status:         200,
		LatencyMS:      42,
		CaptureMode:    "usage",
		TokensCaptured: true,
	})
	got := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Fatalf("output is not JSON: %q", got)
	}
	var entry RequestLog
	if err := json.Unmarshal([]byte(got), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if entry.RequestID != 1 || entry.Method != "POST" || entry.Model != "gpt-4o" || entry.Status != 200 || entry.LatencyMS != 42 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestWriterDisabled(t *testing.T) {
	var buf bytes.Buffer
	w := Disabled()
	w.Request("should not appear\n")
	w.Response("should not appear\n")
	if buf.Len() != 0 {
		t.Fatalf("disabled writer wrote %q", buf.String())
	}
}
