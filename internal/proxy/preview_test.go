package proxy

import "testing"

func TestResponsePreviewLimitsOutput(t *testing.T) {
	preview := NewResponsePreview(5)
	preview.Observe([]byte("hello world"))
	if got := preview.String(); got != "hello" {
		t.Fatalf("preview = %q, want hello", got)
	}
}

func TestResponsePreviewHandlesNil(t *testing.T) {
	var preview *ResponsePreview
	if got := preview.String(); got != "" {
		t.Fatalf("preview = %q, want empty", got)
	}
}
