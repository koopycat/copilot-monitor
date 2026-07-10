## Why

When debugging routing issues, policy behavior, or upstream failures, the
structured request log does not capture enough detail. Request bodies, full
response headers, and provider prefix resolution are all stripped. Users
currently have no built-in way to trace exactly what the proxy sees and
forwards, forcing them to fall back on external packet capture or upstream
service logs. A raw debug mode fills this gap for local troubleshooting.

## What Changes

- Add a `--raw-log <path>` flag to the `run` command that, when set, writes one
  JSON record per proxied request to the specified file.
- Each record captures: request ID, timestamp, HTTP method, original URL path,
  stripped provider prefix, resolved route endpoint, upstream host, request body
  (raw bytes, truncated at a conservatively low limit for privacy), response
  status, response headers (redacted), latency, and any routing or compression
  decisions.
- Raw logging is opt-in and off by default. The flag is only valid with `run`
  (not `serve`).

## Capabilities

### Modified Capabilities

- `proxy`: Add raw debug logging capability — a new `--raw-log` flag that writes
  per-request JSON records to a file. Includes a new requirement with scenarios
  for activation, content boundaries, and header redaction.

## Impact

- `internal/cli/run.go`: new flag definition and validation
- `internal/proxy/server.go`: new `RawLogger` struct (or inline via
  `UsageDebugLogger` convention), called at end of `ServeHTTP`
- `internal/proxy/usage_debug.go`: existing pattern to follow (JSONL encoder,
  mutex, redaction)
- No database schema changes, no API changes, no dashboard changes
