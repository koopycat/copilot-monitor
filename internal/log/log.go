package log

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"
)

// LogFormat controls the output format of log entries.
type LogFormat string

const (
	FormatJSON  LogFormat = "json"
	FormatHuman LogFormat = "human"
)

// RequestLog is the structured log entry emitted per proxied request.
type RequestLog struct {
	RequestID        uint64  `json:"request_id"`
	Method           string  `json:"method"`
	Path             string  `json:"path"`
	Upstream         string  `json:"upstream"`
	Model            string  `json:"model"`
	Status           int     `json:"status"`
	LatencyMS        int64   `json:"latency_ms"`
	CaptureMode      string  `json:"capture_mode"`
	TokensCaptured   bool    `json:"tokens_captured"`
	UsageMissing     bool    `json:"usage_missing,omitempty"`
	Error            string  `json:"error,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	CachedTokens     int     `json:"cached_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	Endpoint         string  `json:"endpoint,omitempty"`
	Provider         string  `json:"provider,omitempty"`
}

// Writer wraps an io.Writer and adds ANSI color when the output is a terminal.
// Respects NO_COLOR and TERM=dumb conventions.
type Writer struct {
	w         io.Writer
	colors    bool
	enabled   bool
	format    LogFormat
	mu        sync.Mutex
	ttyFile   *os.File
	termWidth int

	// Running totals
	headerPrinted         bool
	requestCount          int64
	totalPromptTokens     int64
	totalCompletionTokens int64
	totalCost             float64
	startTime             time.Time
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
	lw := &Writer{w: w, format: format, startTime: time.Now()}
	if f, ok := w.(*os.File); ok && isTerminal(f) {
		if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
			lw.colors = true
		}
		lw.ttyFile = f
		lw.termWidth = TerminalWidth(f)
	} else {
		lw.termWidth = 80
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
		w.requestCount++
		if rl.TokensCaptured {
			w.totalPromptTokens += int64(rl.PromptTokens)
			w.totalCompletionTokens += int64(rl.CompletionTokens)
			w.totalCost += rl.CostUSD
		}

		header := w.renderHeader()
		line := w.formatLine(rl)

		if w.colors {
			// ANSI sticky header: overwrite previous header + request line in place.
			if w.headerPrinted {
				fmt.Fprintf(w.w, "\x1b[2A\x1b[2K%s\n\x1b[2K%s\n", header, line)
			} else {
				fmt.Fprintf(w.w, "%s\n%s\n", header, line)
				w.headerPrinted = true
			}
		} else {
			fmt.Fprintf(w.w, "%s\n%s\n", header, line)
		}
	}
}

// formatLine builds the column-aligned request line.
func (w *Writer) formatLine(rl RequestLog) string {
	width := w.termWidth
	if width < 40 {
		width = 40
	}

	method := rl.Method
	path := truncatePath(rl.Path, 20)
	endpoint := rl.Endpoint
	if endpoint == "" {
		endpoint = "-"
	}
	upstream := truncateStr(rl.Upstream, 15)
	model := rl.Model
	if model == "" {
		model = "-"
	}
	tokens := "--"
	cost := "--"
	if rl.TokensCaptured {
		tokens = fmt.Sprintf("⬆ %s ⬇ %s", formatHumanCount(rl.PromptTokens), formatHumanCount(rl.CompletionTokens))
		cost = formatCost(rl.CostUSD)
	} else if rl.UsageMissing {
		tokens = "miss"
		cost = "--"
	}
	latency := formatLatency(rl.LatencyMS)

	statusStr := fmt.Sprintf("%d", rl.Status)

	if w.colors {
		return fmt.Sprintf("%s%*s%s %s%*s%s %s%*s%s %s%*s%s %s%s%s%s %s%*s%s %s%*s%s %s%*s%s %s%*s%s",
			darkGray, -4, method, reset,
			darkGray, -12, path, reset,
			blue, -8, endpoint, reset,
			darkGray, -12, upstream, reset,
			bold, white, model, reset,
			w.statusColor(rl.Status), 3, statusStr, reset,
			w.latencyColor(rl.LatencyMS), 7, latency, reset,
			lightCyan, 14, tokens, reset,
			w.costColor(rl.CostUSD), 7, cost, reset,
		)
	}
	return fmt.Sprintf("%4s %-12s %-8s %-12s %s %3s %7s %14s %7s",
		method, path, endpoint, upstream, model, statusStr, latency, tokens, cost)
}

// renderHeader renders the running totals header line.
func (w *Writer) renderHeader() string {
	uptime := formatDuration(time.Since(w.startTime))
	header := fmt.Sprintf("── copilot-monitor ──  %d req  ⬆ %s tok ⬇ %s tok  %s  %s ──",
		w.requestCount,
		formatHumanCount(int(w.totalPromptTokens)),
		formatHumanCount(int(w.totalCompletionTokens)),
		formatCost(w.totalCost),
		uptime,
	)
	if w.colors {
		return faint + header + reset
	}
	return header
}

// statusColor returns the ANSI color for an HTTP status code.
func (w *Writer) statusColor(code int) string {
	switch {
	case code >= 500:
		return red
	case code >= 400:
		return yellow
	case code >= 300:
		return cyan
	case code >= 200:
		return green
	default:
		return reset
	}
}

// latencyColor returns the ANSI color for latency.
func (w *Writer) latencyColor(ms int64) string {
	switch {
	case ms > 10000:
		return red
	case ms > 2000:
		return yellow
	default:
		return darkGray
	}
}

// costColor returns the ANSI color for a cost value.
func (w *Writer) costColor(v float64) string {
	switch {
	case v >= 1.0:
		return bold + white
	case v >= 0.10:
		return yellow
	case v >= 0.01:
		return green
	default:
		return darkGray
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

// Intermediate calls are no-ops in human format to avoid breaking
// the single-line-per-request model. Warn and store-level Error remain active.
func (w *Writer) Request(format string, args ...any)  {} // no-op: suppressed in beautiful mode
func (w *Writer) Response(format string, args ...any) {} // no-op: suppressed in beautiful mode
func (w *Writer) Error(format string, args ...any) {
	if w.format == FormatHuman {
		// Error calls for non-request events still emit (e.g., store errors).
		// Request-level errors are captured in RequestLog already.
		w.colored(red, "✗", format, args...)
	}
}
func (w *Writer) Info(format string, args ...any) {} // no-op: suppressed in beautiful mode
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

// Human number formatting

func formatHumanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatLatency(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

func formatCost(v float64) string {
	if v <= 0 {
		return "--"
	}
	if v < 0.01 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", v)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		s := int(math.Mod(d.Seconds(), 60))
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), s)
	}
	h := int(d.Hours())
	m := int(math.Mod(d.Minutes(), 60))
	return fmt.Sprintf("%dh%dm", h, m)
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "…" + path[len(path)-maxLen+1:]
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
