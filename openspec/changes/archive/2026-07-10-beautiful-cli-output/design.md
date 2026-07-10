## Context

The `run` command currently defaults to `--log-format json`, emitting one JSON
blob per request to stderr. A `human` format exists but renders bare `key=value`
lines that are functional but uninspiring. When the live session tail is active
(default on TTY), per-request logging is completely silenced.

The existing `log.Writer` already has ANSI color plumbing (`colored` method,
color constants, TTY detection, `NO_COLOR`/`TERM=dumb` honoring). The proxy
handler already calls structured methods on the writer: `Request`, `Response`,
`Error`, `Info`, `Warn`, and `RequestLogEntry`.

## Goals / Non-Goals

**Goals:**

- Beautiful, colored, column-aligned per-request output by default (no flags
  needed).
- Sticky header with running totals (requests, tokens, cost, uptime) that stays
  at the top of the terminal.
- Default `--log-format` changes from `json` to `human`.
- Zero new dependencies -- ANSI escape codes only.
- Works in both TTY and pipe/redirect contexts (graceful degradation).

**Non-Goals:**

- Changing the live session tail behavior or display format.
- Adding interactive controls (no arrow keys, menus, or mouse input).
- Adding sparklines, progress bars, or animated spinners.
- Changing the JSON format output in any way.

## Decisions

### 1. Enrich RequestLog rather than create new methods

**Decision:** Add optional fields (`PromptTokens`, `CompletionTokens`,
`CachedTokens`, `CostUSD`, `Endpoint`) to the existing `RequestLog` struct. The
human format rendering in `RequestLogEntry` uses these fields when present. The
JSON format ignores them (they are `omitempty` and never appear in JSON output
since they are only set when the proxy handler fills them in).

**Rationale:** The proxy handler already calls `RequestLogEntry` at the end of
every request. Enriching this single struct avoids adding new method calls and
keeps the output point single. The JSON output is unchanged because the new
fields are zero-valued for the JSON path.

**Alternatives considered:**

- New `RichRequestLog` struct alongside existing one: adds duplication and
  confusion about which to use.
- Separate `BeautifulLine` method: two output points to maintain and coordinate.

### 2. Suppress intermediate human-format calls when beautiful mode is active

**Decision:** In human format, the `Request`, `Response`, `Error`, and `Info`
methods become no-ops. Only `RequestLogEntry` produces output. `Warn` and
`Error` for non-request events (e.g., store errors, policy load failures) still
emit.

**Rationale:** The intermediate calls are noisy and break the
single-line-per-request model. Warnings and errors unrelated to request flow are
important diagnostics that should still appear.

### 3. Sticky header rendered via ANSI cursor manipulation

**Decision:** The sticky header is rendered as an ANSI region above the
scrolling log. Before printing a new request line, the writer:

1. Saves cursor position
2. Moves up to header line
3. Clears and reprints the header
4. Restores cursor position
5. Prints the new request line

This only activates when stderr is a TTY and `--log-format human` and
`--no-live`. When `--no-live` is not set, the live tail takes over and the
header is not rendered.

**Rationale:** This is the standard approach for sticky headers in terminal
applications (same pattern used by `top`, `htop`, build tools with progress
bars, and the existing `clearTail`/`writeLines` in `live.go`).

**Alternatives considered:**

- Redraw the entire screen: flickers and is overkill for a single header line.
- Use a separate terminal pane: requires terminal multiplexing, not appropriate
  for a CLI tool.
- Emit header repeats inline: clutters the scrollback.

### 4. Column layout and field widths

**Decision:** Use fixed-width columns aligned to the right edge for numeric
fields. The layout is:

```
{METHOD} {PATH} {ENDPOINT} {UPSTREAM} {MODEL} {STATUS} {LATENCY} {TOKENS} {COST}
```

- METHOD and PATH: dimmed, right-aligned path truncated to fit
- ENDPOINT: blue, human-readable label
- MODEL: bold white
- STATUS: color-coded (2xx=green, 3xx=cyan, 4xx=yellow, 5xx=red)
- LATENCY: dimmed, right-aligned (e.g., `1.2s`, `843ms`)
- TOKENS: `⬇1.2k ⬆342` or `in:1.2k out:342` depending on unicode support
- COST: right-aligned with color gradient (cheap=green, medium=yellow,
  expensive=bold)

Fields overflow gracefully: narrow terminals show fewer columns (drop upstream,
shorten path).

**Rationale:** Fixed columns with right-aligned numbers make scanning easy.
Color coding status and cost gives instant visual feedback. Column width should
adapt to terminal width when detectable.

### 5. Sticky header content

**Decision:** The header displays in a single dimmed line:

```
── copilot-monitor ──  12 req  ⬇15.3k tok ⬆4.1k tok  $0.47  4m32s ──
```

Fields: request count, input tokens, output tokens, total cost, uptime. The
header is separated from request lines by a visual border (dimmed dashes).

**Rationale:** This mirrors common monitoring tools and gives the "joyful
dashboard" feel the user wants. Token and cost numbers are human-formatted
(1.2k, 1.5M). Uptime shows gradually: seconds, then minutes, then hours.

## Risks / Trade-offs

- **[Pipe/redirect still works]**: When stderr is piped, ANSI codes are stripped
  and the header is a plain line that repeats. This is acceptable -- the output
  is still human-readable and grep-friendly.
- **[Concurrent output race]**: The proxy handles concurrent requests. The
  `Writer.mu` mutex serializes output, so header updates and request lines don't
  interleave. This means very high concurrency might show slight output delays,
  but acceptable for a local dev tool.
- **[Breaking default change]**: Changing the default from `json` to `human`
  breaks scripts that relied on the default JSON output. Users must add
  `--log-format json`. This is documented in the proposal and the flag help text
  clearly states the default.
- **[Terminal width detection]**: On some terminals (CI, Docker without TTY),
  width detection returns 0 or a default (80). The formatter handles this by
  using conservative column widths. On a real TTY, `$COLUMNS` or the TIOCGWINSZ
  ioctl gives accurate width.
