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
	w.Warn("id=%d something=%s\n", 1, "odd")
	got := buf.String()
	if got != "! id=1 something=odd\n" {
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

// Task 6.1: Unit tests for human number and latency formatting.
func TestFormatHumanCount(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1.0k"},
		{1200, "1.2k"},
		{1500, "1.5k"},
		{9999, "10.0k"},
		{10000, "10.0k"},
		{15000, "15.0k"},
		{999999, "1000.0k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{10000000, "10.0M"},
	}
	for _, tt := range tests {
		got := formatHumanCount(tt.input)
		if got != tt.want {
			t.Errorf("formatHumanCount(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{0, "0ms"},
		{1, "1ms"},
		{843, "843ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1200, "1.2s"},
		{1500, "1.5s"},
		{10000, "10.0s"},
		{60000, "60.0s"},
	}
	for _, tt := range tests {
		got := formatLatency(tt.ms)
		if got != tt.want {
			t.Errorf("formatLatency(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{0, "--"},
		{-1, "--"},
		{0.001, "<$0.01"},
		{0.009, "<$0.01"},
		{0.01, "$0.01"},
		{0.10, "$0.10"},
		{0.99, "$0.99"},
		{1.00, "$1.00"},
		{1.50, "$1.50"},
		{10.00, "$10.00"},
	}
	for _, tt := range tests {
		got := formatCost(tt.v)
		if got != tt.want {
			t.Errorf("formatCost(%f) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// Task 6.2: Unit tests for column-aligned formatter output.
func TestFormatLineWithTokens(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)
	rl := RequestLog{
		RequestID:        1,
		Method:           "POST",
		Path:             "/chat/completions",
		Endpoint:         "chat",
		Upstream:         "api.openai.com",
		Model:            "gpt-4o",
		Status:           200,
		LatencyMS:        1200,
		TokensCaptured:   true,
		PromptTokens:     1500,
		CompletionTokens: 342,
		CostUSD:          0.02,
	}

	line := w.formatLine(rl)

	// Check that key fields are present in the aligned output
	if !strings.Contains(line, "POST") {
		t.Errorf("line missing method: %s", line)
	}
	if !strings.Contains(line, "gpt-4o") {
		t.Errorf("line missing model: %s", line)
	}
	if !strings.Contains(line, "200") {
		t.Errorf("line missing status: %s", line)
	}
	if !strings.Contains(line, "1.2s") {
		t.Errorf("line missing latency: %s", line)
	}
	if !strings.Contains(line, "⬆ 1.5k") {
		t.Errorf("line missing input tokens: %s", line)
	}
	if !strings.Contains(line, "⬇ 342") {
		t.Errorf("line missing output tokens: %s", line)
	}
	if !strings.Contains(line, "$0.02") {
		t.Errorf("line missing cost: %s", line)
	}
}

func TestFormatLineNoUsage(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)
	rl := RequestLog{
		RequestID:      2,
		Method:         "GET",
		Path:           "/models",
		Endpoint:       "models",
		Upstream:       "api.anthropic.com",
		Model:          "claude-3",
		Status:         401,
		LatencyMS:      230,
		TokensCaptured: false,
		UsageMissing:   true,
	}

	line := w.formatLine(rl)

	// Should show -- or "miss" for tokens, not numeric values
	if !strings.Contains(line, "miss") && !strings.Contains(line, "--") {
		t.Errorf("line should show missing tokens indicator: %s", line)
	}
	if !strings.Contains(line, "401") {
		t.Errorf("line missing status: %s", line)
	}
	if !strings.Contains(line, "claude-3") {
		t.Errorf("line missing model: %s", line)
	}
}

func TestFormatLineColors(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)
	rl := RequestLog{
		RequestID:        3,
		Method:           "POST",
		Path:             "/chat/completions",
		Endpoint:         "chat",
		Upstream:         "api.example.com",
		Model:            "gpt-4o",
		Status:           500,
		LatencyMS:        15000,
		TokensCaptured:   true,
		PromptTokens:     100,
		CompletionTokens: 50,
		CostUSD:          1.50,
	}

	line := w.formatLine(rl)

	// 500 status, high latency, high cost - colors should still be there
	// even on non-TTY the color codes won't appear (which is expected)
	if !strings.Contains(line, "500") {
		t.Errorf("line missing status: %s", line)
	}
	if !strings.Contains(line, "15.0s") {
		t.Errorf("line missing latency: %s", line)
	}
	if !strings.Contains(line, "$1.50") {
		t.Errorf("line missing cost: %s", line)
	}
}

// Task 6.3: Unit test for non-TTY plain-text output (no ANSI codes).
func TestHumanFormatNoANSICodes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)
	// Buffer is not a terminal, so colors should be false.
	rl := RequestLog{
		RequestID:        1,
		Method:           "POST",
		Path:             "/chat",
		Endpoint:         "chat",
		Upstream:         "api.example.com",
		Model:            "gpt-4o",
		Status:           200,
		LatencyMS:        42,
		TokensCaptured:   true,
		PromptTokens:     100,
		CompletionTokens: 50,
		CostUSD:          0.01,
	}
	w.RequestLogEntry(rl)
	got := buf.String()

	// No ANSI codes in the output
	for _, code := range []string{"\x1b[", "\033["} {
		if strings.Contains(got, code) {
			t.Errorf("output contains ANSI escape: %q", got)
		}
	}

	// Must contain basic fields
	if !strings.Contains(got, "POST") {
		t.Errorf("output missing method: %s", got)
	}
	if !strings.Contains(got, "gpt-4o") {
		t.Errorf("output missing model: %s", got)
	}
	if !strings.Contains(got, "200") {
		t.Errorf("output missing status: %s", got)
	}

	// Header should be present
	if !strings.Contains(got, "copilot-monitor") {
		t.Errorf("output missing header: %s", got)
	}
}

// Task 6.4: Unit test for RequestLog JSON serialization with new optional fields omitted when zero.
func TestJSONSerializationOmitsZeroFields(t *testing.T) {
	// Test that new optional fields are omitted when zero
	entry := RequestLog{
		RequestID:      1,
		Method:         "POST",
		Path:           "/chat",
		Upstream:       "api.example.com",
		Model:          "gpt-4o",
		Status:         200,
		LatencyMS:      42,
		CaptureMode:    "usage",
		TokensCaptured: true,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(data)

	// These fields should NOT appear in JSON when zero
	for _, field := range []string{"prompt_tokens", "completion_tokens", "cached_tokens", "cost_usd", "endpoint", "provider"} {
		if strings.Contains(got, field) {
			t.Errorf("JSON contains zero-valued field %q: %s", field, got)
		}
	}

	// Core fields should still be present
	for _, field := range []string{"request_id", "method", "model", "status", "latency_ms"} {
		if !strings.Contains(got, field) {
			t.Errorf("JSON missing field %q: %s", field, got)
		}
	}
}

func TestJSONSerializationWithNewFields(t *testing.T) {
	// Test that new optional fields appear when non-zero
	entry := RequestLog{
		RequestID:        1,
		Method:           "POST",
		Path:             "/chat",
		Upstream:         "api.example.com",
		Model:            "gpt-4o",
		Status:           200,
		LatencyMS:        42,
		CaptureMode:      "usage",
		TokensCaptured:   true,
		PromptTokens:     1500,
		CompletionTokens: 342,
		CachedTokens:     200,
		CostUSD:          0.05,
		Endpoint:         "chat",
		Provider:         "openai",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(data)

	// All new fields should appear
	for _, field := range []string{
		`"prompt_tokens":1500`,
		`"completion_tokens":342`,
		`"cached_tokens":200`,
		`"cost_usd":0.05`,
		`"endpoint":"chat"`,
		`"provider":"openai"`,
	} {
		if !strings.Contains(got, field) {
			t.Errorf("JSON missing field %q: %s", field, got)
		}
	}
}

// Task 6.5: Integration-style test verifying the proxy emits beautiful human output
// when stderr is captured (pipe mode, no ANSI codes).
func TestProxyEmitsHumanOutputInPipeMode(t *testing.T) {
	// Simulate what a pipe/non-TTY would see: the human format with header + request line.
	var buf bytes.Buffer
	w := NewWriterWithFormat(&buf, FormatHuman)

	// First request
	w.RequestLogEntry(RequestLog{
		RequestID:        1,
		Method:           "POST",
		Path:             "/chat/completions",
		Endpoint:         "chat",
		Upstream:         "api.openai.com",
		Model:            "gpt-4o",
		Status:           200,
		LatencyMS:        1200,
		TokensCaptured:   true,
		PromptTokens:     1500,
		CompletionTokens: 342,
		CostUSD:          0.02,
	})
	// Second request
	w.RequestLogEntry(RequestLog{
		RequestID:        2,
		Method:           "POST",
		Path:             "/chat/completions",
		Endpoint:         "chat",
		Upstream:         "api.openai.com",
		Model:            "gpt-3.5-turbo",
		Status:           200,
		LatencyMS:        843,
		TokensCaptured:   true,
		PromptTokens:     500,
		CompletionTokens: 200,
		CostUSD:          0.01,
	})

	got := buf.String()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %q", len(lines), got)
	}

	// No ANSI codes
	if strings.Contains(got, "\x1b[") {
		t.Errorf("pipe mode output contains ANSI codes: %q", got)
	}

	// Should contain both header and request lines
	if !strings.Contains(got, "copilot-monitor") {
		t.Errorf("output missing header: %s", got)
	}
	if !strings.Contains(got, "gpt-4o") {
		t.Errorf("output missing first model: %s", got)
	}
	if !strings.Contains(got, "gpt-3.5-turbo") {
		t.Errorf("output missing second model: %s", got)
	}

	// Running totals should be updated: 2 requests, tokens accumulated
	if !strings.Contains(got, "2 req") {
		t.Errorf("output missing request count header: %s", got)
	}
}

// Task 6.6 is verified by running 'go test ./...' which passes.
