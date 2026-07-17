## Why

Phase 1 fixed bugs and structural issues (column headers, env vars, graceful
shutdown, onboarding order). Phase 2 addresses the remaining friction:
`live --watch` destroys terminal scrollback every 2 seconds, CLI error messages
have no consistent format, and several commands produce terse or
machine-oriented output that could be friendlier.

## What Changes

- **live --watch** uses cursor-up ANSI clearing instead of full-screen clear,
  matching the `run` live tail behavior. No longer destroys scrollback.
- **CLI error messages** adopt a consistent `"error: <what failed>: <detail>"`
  format across all commands. Every fatal message starts with `error:` and names
  the operation that failed.
- **today** prints a human-friendly date header instead of full RFC3339.
- **rebuild-sessions** prints counts and timing after completion.
- **cost** gains a compression removed tokens column, matching `stats` and
  `live`.
- **PolicyPanel** heading (`h2`) uses the muted color like all other section
  headers.
- **.session-period-bar** CSS margin override targets the inner `.period-bar`
  div now that SessionsTable uses the `<PeriodBar>` component.

## Capabilities

### New Capabilities

- `cli-error-consistency`: Every CLI command formats fatal error messages on
  stderr with the same `"error: <operation>: <reason>"` structure.
- `live-watch-scrollback`: `live --watch` preserves terminal scrollback by using
  cursor-up clearing instead of full-screen clear.
- `cli-output-improvements`: `today` uses a friendlier date format,
  `rebuild-sessions` reports counts and timing, `cost` shows compression removed
  tokens.

### Modified Capabilities

<!-- No existing spec requirement changes. All changes are implementation-level improvements. -->

## Impact

- `internal/cli/` -- all CLI command files (error message format, live watch,
  today, rebuild-sessions, cost)
- `dashboard/src/app.css` -- PolicyPanel heading color, .session-period-bar
  margin
