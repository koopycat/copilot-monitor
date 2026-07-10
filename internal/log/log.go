package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// LogFormat controls the output format of log entries.
type LogFormat string

const (
	FormatJSON  LogFormat = "json"
	FormatHuman LogFormat = "human"
)

// RequestLog is the structured log entry emitted per proxied request.
type RequestLog struct {
	RequestID      uint64 `json:"request_id"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	Upstream       string `json:"upstream"`
	Model          string `json:"model"`
	Status         int    `json:"status"`
	LatencyMS      int64  `json:"latency_ms"`
	CaptureMode    string `json:"capture_mode"`
	TokensCaptured bool   `json:"tokens_captured"`
	UsageMissing   bool   `json:"usage_missing,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Writer wraps an io.Writer and adds ANSI color when the output is a terminal.
// Respects NO_COLOR and TERM=dumb conventions.
type Writer struct {
	w       io.Writer
	colors  bool
	enabled bool
	format  LogFormat
	mu      sync.Mutex
}

const (
	reset      = "\033[0m"
	bold       = "\033[1m"
	faint      = "\033[2m"
	red        = "\033[31m"
	green      = "\033[32m"
	yellow     = "\033[33m"
	blue       = "\033[34m"
	magenta    = "\033[35m"
	cyan       = "\033[36m"
	white      = "\033[97m"
	darkGray   = "\033[90m"
	lightRed   = "\033[91m"
	lightGreen = "\033[92m"
	lightCyan  = "\033[96m"
)

// NewWriter returns a Writer for the given io.Writer.
// Colors are enabled only when w is a terminal and NO_COLOR is not set and TERM is not dumb.
func NewWriter(w io.Writer) *Writer {
	return NewWriterWithFormat(w, FormatJSON)
}

// NewWriterWithFormat returns a Writer with the given log format.
func NewWriterWithFormat(w io.Writer, format LogFormat) *Writer {
	lw := &Writer{w: w, format: format}
	if f, ok := w.(*os.File); ok && isTerminal(f) {
		if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
			lw.colors = true
		}
	}
	lw.enabled = true
	return lw
}

// Disabled returns a Writer that discards all output.
func Disabled() *Writer { return &Writer{} }

// RequestLogEntry emits a structured log line for a completed proxy request.
func (w *Writer) RequestLogEntry(rl RequestLog) {
	if !w.enabled || w.w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.format == FormatJSON {
		data, err := json.Marshal(rl)
		if err != nil {
			return
		}
		fmt.Fprintln(w.w, string(data))
	} else {
		// Human format
		usageInfo := ""
		if rl.TokensCaptured {
			usageInfo = " usage=yes"
		} else if rl.UsageMissing {
			usageInfo = " usage_missing=yes"
		} else {
			usageInfo = " usage=no"
		}
		errInfo := ""
		if rl.Error != "" {
			errInfo = fmt.Sprintf(" error=%q", rl.Error)
		}
		fmt.Fprintf(w.w, "id=%d method=%s path=%q upstream=%s model=%q status=%d latency_ms=%d%s%s\n",
			rl.RequestID, rl.Method, rl.Path, rl.Upstream, rl.Model, rl.Status, rl.LatencyMS, usageInfo, errInfo)
	}
}

func (w *Writer) write(format string, args ...any) {
	if !w.enabled || w.w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.w, format, args...)
}

func (w *Writer) colored(color, label, format string, args ...any) {
	if w.colors {
		w.write("%s%s%s "+format, append([]any{color, label, reset}, args...)...)
	} else {
		w.write("%s "+format, append([]any{label}, args...)...)
	}
}

func (w *Writer) Request(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(darkGray, "→", format, args...)
	}
}
func (w *Writer) Response(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(lightGreen, "←", format, args...)
	}
}
func (w *Writer) Error(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(lightRed, "✗", format, args...)
	}
}
func (w *Writer) Info(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(faint, "·", format, args...)
	}
}
func (w *Writer) Websocket(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(magenta, "⇄", format, args...)
	}
}
func (w *Writer) Ping(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(faint, "·", format, args...)
	}
}
func (w *Writer) Warn(format string, args ...any) {
	if w.format == FormatHuman {
		w.colored(yellow, "!", format, args...)
	}
}
