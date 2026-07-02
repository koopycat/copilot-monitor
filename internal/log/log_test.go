package log

import (
	"bytes"
	"testing"
)

func TestWriterNoColorsForBuffer(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Request("id=%d method=%s\n", 1, "GET")
	got := buf.String()
	if got != "→ id=1 method=GET\n" {
		t.Fatalf("got %q", got)
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
