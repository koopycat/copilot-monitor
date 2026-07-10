## Why

When running `copilot-monitor run --no-live`, every request dumps a JSON blob to
stderr -- machine-readable but joyless. A human watching the proxy should see a
beautiful, colored, lively terminal display that makes LLM API activity visible
and delightful, not an opaque stream of JSON.

## What Changes

- Change the default `--log-format` from `json` to `human`.
- Replace the current bare `key=value` human format with a rich, colored,
  per-request display that includes model name, endpoint, status (color-coded),
  latency, token counts, and cost -- all in a beautifully aligned layout.
- Add a sticky header line showing running totals (request count, total tokens,
  total cost) that updates in place using ANSI escape codes, giving a joyful
  "dashboard" feeling even without the live tail.
- Each request scrolls by as a single compact line with distinct colors for
  different elements: bold model and endpoint, color-coded HTTP status, subtle
  dimmed metadata.
- **BREAKING**: The default log format changes from `json` to `human` -- anyone
  relying on the previous default JSON output must now pass `--log-format json`
  explicitly.

## Capabilities

### New Capabilities

- `cli-output`: Beautiful, colored, human-readable terminal output for the proxy
  `run` command, replacing the previous bare key=value format with a rich
  per-request display and running totals.

### Modified Capabilities

- `proxy`: The live session tail behavior remains unchanged; the log format
  default changes from `json` to `human` and the human format rendering is
  redesigned.

## Impact

- `internal/log/log.go`: Rewrite the `human` format rendering with rich colored
  output, running totals, and sticky header.
- `internal/cli/run.go`: Change `--log-format` default from `"json"` to
  `"human"`.
- `internal/proxy/server.go`: Minor updates to ensure the proxy's log calls
  align with the new output contract (existing methods like `Request`,
  `Response`, `Error`, `Info`, `Warn` already exist as entry points).
- No API or database changes.
- No new dependencies -- uses existing ANSI escape code approach already in the
  codebase.
