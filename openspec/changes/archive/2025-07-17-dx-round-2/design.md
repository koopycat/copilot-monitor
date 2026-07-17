## Context

Phase 1 unified column headers, added env var support, fixed bugs (help exit
code, serve shutdown), improved onboarding, and cleaned up dashboard component
reuse. Phase 2 addresses remaining papercuts identified in the DX audit.

The changes are small, localized, and non-breaking. Each touches at most one
file. No new dependencies, no schema changes, no API changes.

## Goals / Non-Goals

**Goals:**

- `live --watch` preserves terminal scrollback (uses cursor-up clearing like
  `run`'s live tail)
- Every CLI error message on stderr follows a consistent
  `"error: <operation>: <detail>"` format
- `today` prints a user-friendly date like "Usage for July 17, 2025" instead of
  RFC3339
- `rebuild-sessions` prints session count and elapsed time after completion
- `cost` shows a compression removed tokens column (matching `stats` and `live`)
- PolicyPanel `h2` uses `--muted` color like every other section heading
- `.session-period-bar` margin override works correctly with the `<PeriodBar>`
  component

**Non-Goals:**

- `run` startup output stream split (stdout vs stderr) -- requires broader
  design discussion
- `--no-live` flag polarity change -- would be a breaking CLI change
- Dashboard auto-refresh countdown indicator -- involves Svelte store and timer
  refactoring
- Website download platform parity -- marketing page, not shipped code
- Port number documentation -- docs-only, not blocking

## Decisions

### 1. Error message format: `"error: <operation>: <detail>"`

**Chosen:** `fmt.Fprintf(stderr, "error: %s: %v\n", op, err)` consistently
across all commands.

**Alternatives considered:**

- `"<operation> failed: %v"` -- verbose, inconsistent with `"error:"` prefix
  already used in `run.go`
- `log.Fatalf` -- doesn't let us control exit codes granularly
- Custom error type -- overengineered for this scale

The `"error:"` prefix is already used in `run.go:95`. Extending it everywhere
creates a grep-friendly pattern for scripts: `2>&1 | grep '^error:'`.

### 2. `live --watch` ANSI clearing: cursor-up instead of full-screen

**Chosen:** Replace `\x1b[2J\x1b[H` (full-screen clear + home) with the same
cursor-up pattern used in `run.go`'s `startLiveTail`: `\x1b[%dA\x1b[0J` (move up
N lines, clear to end of screen).

The `live --watch` render writes a known number of lines. Track previous line
count, move up by that amount before each re-render. This is exactly what
`run`'s live tail already does in `clearTail`/`writeLines`.

### 3. `today` date format: month day, year

**Chosen:** `"Usage for January 2, 2006"` using Go's
`time.Format("January 2, 2006")`.

This is more scannable than `2006-01-02T00:00:00Z` and doesn't lose precision --
the time is always local midnight.

### 4. `rebuild-sessions` output: count and duration

**Chosen:** Print `"rebuilt <N> sessions from <M> requests in <duration>"` on
stdout. Track elapsed time with `time.Since(start)` and count rebuilt sessions
from the return value.

The store function `RebuildSessions` returns `(int, error)` -- the int is the
session count. Request count can be derived from a quick count query or by
counting before/after.

### 5. `cost` compression column

**Chosen:** Add `TOKENS_REMOVED` as the rightmost column in the cost table,
matching `stats.go`'s implementation. When `CompressionRemovedTokens == 0`, show
`"-"`. Source the data from the existing `modelStats.CompressionRemovedTokens`
field which is already populated in `store.Stats()`.

### 6. PolicyPanel heading color: `--muted`

**Chosen:** Remove the `color: var(--ink)` override from `.policy-panel h2`,
letting it inherit the global `h2 { color: var(--muted) }`.

### 7. `.session-period-bar` margin: target inner `.period-bar`

**Chosen:** Change `.session-period-bar { margin: 0; }` to
`.session-period-bar .period-bar { margin: 0; }`. Since `<PeriodBar>` now
renders a nested `<div class="period-bar">`, the margin override must target the
child to zero out the default `margin-bottom: var(--s3)` from `.period-bar`.

## Risks / Trade-offs

- **Error message format change** -- script parsers that match on specific error
  strings will break. Risk is low because error messages already varied between
  commands and no parsing contract was documented.
- **`live --watch` line count tracking** -- if the render output changes line
  count between refreshes (e.g., model list grows), the cursor-up could briefly
  leave artifacts. Mitigation: the `writeLines` approach from `run.go` already
  handles this by tracking the previous line count per render.
