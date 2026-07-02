package log

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Writer wraps an io.Writer and adds ANSI color when the output is a terminal.
// Respects NO_COLOR and TERM=dumb conventions.
type Writer struct {
	w       io.Writer
	colors  bool
	enabled bool
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
	lw := &Writer{w: w}
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
	w.colored(darkGray, "→", format, args...)
}
func (w *Writer) Response(format string, args ...any) {
	w.colored(lightGreen, "←", format, args...)
}
func (w *Writer) Error(format string, args ...any) {
	w.colored(lightRed, "✗", format, args...)
}
func (w *Writer) Info(format string, args ...any) {
	w.colored(faint, "·", format, args...)
}
func (w *Writer) Websocket(format string, args ...any) {
	w.colored(magenta, "⇄", format, args...)
}
func (w *Writer) Ping(format string, args ...any) {
	w.colored(faint, "·", format, args...)
}
func (w *Writer) Warn(format string, args ...any) {
	w.colored(yellow, "!", format, args...)
}
