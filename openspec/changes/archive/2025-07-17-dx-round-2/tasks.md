## 1. CLI error message consistency

- [x] 1.1 Audit every `fmt.Fprintf(stderr, ...)` call in `internal/cli/` for
      non-error messages (doctor report output, startup banners) to avoid adding
      `"error:"` to non-errors
- [x] 1.2 Rewrite all fatal stderr messages in `stats.go`, `cost.go`,
      `today.go`, `sessions.go`, `live.go`, `export.go`, `inspect.go`,
      `doctor.go`, `rebuild_sessions.go` to use
      `"error: <operation>: <detail>\n"` format
- [x] 1.3 Keep `serve.go` and `run.go` startup/lifecycle messages on stderr
      without `"error:"` prefix (they are operational output, not fatal errors)
- [x] 1.4 Run `just test` and verify `cli_test.go` assertions match updated
      error strings

## 2. live --watch scrollback preservation

- [x] 2.1 In `internal/cli/live.go`, replace `fmt.Fprint(w, "\x1b[2J\x1b[H")` in
      `runLiveWatch` with cursor-up clearing using the same pattern as
      `run.go`'s `clearTail`/`writeLines` helpers
- [x] 2.2 Track the previous line count across watch iterations and move cursor
      up by that count before each re-render
- [x] 2.3 Run `just test` to verify no regressions in live output tests

## 3. CLI output improvements

- [x] 3.1 In `today.go`, change `start.Format(time.RFC3339)` to
      `start.Format("January 2, 2006")` and update the format string to
      `"Usage for %s\n"`
- [x] 3.2 In `rebuild_sessions.go`, track start time before calling
      `st.RebuildSessions()`, capture the returned session count, count total
      requests before rebuild, print
      `"rebuilt <N> sessions from <M> requests in <duration>\n"` on success
- [x] 3.3 In `cost.go`, add `TOKENS_REMOVED` column to the table header and
      rows, sourcing data from `row.CompressionRemovedTokens` (already available
      from `store.Stats()`). Show `"-"` when zero
- [x] 3.4 Run `just test` and update any test assertions that check cost output
      format

## 4. Dashboard polish

- [x] 4.1 In `dashboard/src/app.css`, remove the `color: var(--ink)` line from
      `.policy-panel h2` so it inherits the global `h2 { color: var(--muted) }`
- [x] 4.2 In `dashboard/src/app.css`, change
      `.session-period-bar { margin: 0; }` to
      `.session-period-bar .period-bar { margin: 0; }` to target the nested
      `<PeriodBar>` component div
- [x] 4.3 Run `cd dashboard && pnpm check` to verify no TypeScript/Svelte errors

## 5. Final verification

- [x] 5.1 Run `just test` and confirm all tests pass
- [x] 5.2 Run `just vet` and confirm clean
- [x] 5.3 Run `just build` and confirm full build succeeds
