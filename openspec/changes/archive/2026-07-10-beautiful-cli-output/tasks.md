## 1. Enrich structured log entry

- [x] 1.1 Add optional token, cost, and endpoint fields to `log.RequestLog`
      struct (PromptTokens, CompletionTokens, CachedTokens, CostUSD, Endpoint,
      Provider) with `omitempty` JSON tags so JSON output is unchanged
- [x] 1.2 Update proxy handler (`internal/proxy/server.go`) to populate the new
      fields from `observer.Usage`, `costResult`, and `route.Endpoint` before
      calling `RequestLogEntry`

## 2. Build beautiful human-format rendering

- [x] 2.1 Add terminal width detection to `log.Writer` (check `COLUMNS` env var,
      fallback to TIOCGWINSZ ioctl if stderr is an \*os.File, default 80)
- [x] 2.2 Implement column-aligned request line formatter: method (dimmed), path
      (dimmed), endpoint (blue), upstream (dimmed), model (bold white), status
      (color-coded), latency (right-aligned), tokens (⬇/⬆ with human units),
      cost (right-aligned with color gradient)
- [x] 2.3 Implement color scheme in `RequestLogEntry` human path: 2xx=green,
      3xx=cyan, 4xx=yellow, 5xx=red; latency color:
      normal=dimmed, >2s=yellow, >10s=red; cost color: <$0.01=dimmed,
      <$0.10=green, <$1.00=yellow, >=$1.00=bold white
- [x] 2.4 Implement human number formatting: 1200 → 1.2k, 1500000 → 1.5M;
      latency: 1200ms → 1.2s, 843ms → 843ms
- [x] 2.5 Suppress intermediate calls in human format: make `Request`,
      `Response`, `Error` (request-level), and `Info` no-ops when format is
      `human`; keep `Warn`, `Websocket`, and store-level `Error` active for
      diagnostics

## 3. Sticky running totals header

- [x] 3.1 Add running-totals state to `log.Writer`: requestCount,
      totalPromptTokens, totalCompletionTokens, totalCost, startTime
- [x] 3.2 Accumulate totals in `RequestLogEntry` when in human format
- [x] 3.3 Implement ANSI sticky-header rendering: before each request line, save
      cursor, move up to header line, clear it, redraw header, restore cursor,
      print request line
- [x] 3.4 Format header line:
      `── copilot-monitor ──  N req  ⬇X.X tok ⬆X.X tok  $X.XX  XmXs ──` using
      dimmed styling
- [x] 3.5 Guard sticky header behind TTY check: only render ANSI header when
      stderr is a terminal, `--log-format human`, and `--no-live` is set (header
      is per-request, not compatible with live tail)

## 4. Change default log format

- [x] 4.1 Change `--log-format` flag default in `internal/cli/run.go` from
      `"json"` to `"human"`
- [x] 4.2 Validate flag value in `run.go`: reject values other than `human` and
      `json` with an error listing valid options
- [x] 4.3 Update `run` command help text to document the new default and
      describe the human format

## 5. Non-TTY graceful degradation

- [x] 5.1 Ensure all ANSI escape codes are suppressed when stderr is not a TTY
      (already handled by existing `colors` flag, verify)
- [x] 5.2 Ensure sticky header becomes a plain inline text line (no cursor
      manipulation) when stderr is not a TTY
- [x] 5.3 Ensure column alignment still works in plain-text mode (pad with
      spaces, no color codes)

## 6. Tests

- [x] 6.1 Add unit tests for human number formatting (1.2k, 1.5M) and latency
      formatting
- [x] 6.2 Add unit tests for column-aligned formatter output (verify field
      positions with varying data widths)
- [x] 6.3 Add unit test for non-TTY plain-text output (no ANSI codes in the
      string)
- [x] 6.4 Add unit test for RequestLog JSON serialization with new optional
      fields (verify they are omitted when zero)
- [x] 6.5 Add integration test verifying the proxy emits beautiful human output
      when stderr is captured (pipe mode)
- [x] 6.6 Verify existing tests pass with the default format change (run
      `just all`)
