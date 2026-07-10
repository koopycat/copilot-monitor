## 1. Raw Logger Implementation

- [x] 1.1 Create `internal/proxy/raw_log.go` with `RawLogger` struct (mirroring
      `UsageDebugLogger`: `OpenRawLogger`, `Close`, `Write` with mutex)
- [x] 1.2 Define `RawLogRecord` struct with all fields: `request_id`, `ts`,
      `method`, `path`, `provider`, `endpoint`, `upstream`, `model`, `stream`,
      `request_body` (base64), `request_body_truncated`, `status`, `latency_ms`,
      `response_headers`, `route_matched`, `compression_status`,
      `policy_allowed`
- [x] 1.3 Add `OpenRawLogger` function that creates the file with 0600
      permissions and returns nil when path is empty (same convention as
      `OpenUsageDebugLogger`)
- [x] 1.4 Add `Write` method with mutex lock, JSON encoding, and error handling
      (same pattern as `UsageDebugLogger.Write`)
- [x] 1.5 Add `Close` method (no-op when nil, same convention)

## 2. CLI Flag

- [x] 2.1 Add `--raw-log` string flag to the `run` flag set in
      `internal/cli/run.go`
- [x] 2.2 Call `proxy.OpenRawLogger(*rawLogPath)` after the existing
      `proxy.OpenUsageDebugLogger` call
- [x] 2.3 Print startup privacy warning to stderr when `--raw-log` is set: "raw
      debug logging is enabled: request bodies (up to 1024 bytes) are written to
      <path>. This file may contain source code and prompts. Treat it as
      sensitive."
- [x] 2.4 Add `defer rawLogger.Close()` alongside the existing
      `defer usageDebug.Close()`

## 3. Proxy Integration

- [x] 3.1 Add `rawLogger *RawLogger` field to the `Handler` struct in
      `internal/proxy/server.go`
- [x] 3.2 Add `SetRawLogger(rl *RawLogger)` method on `Handler` (or pass via
      constructor)
- [x] 3.3 Add `writeRawLog` method on `Handler` that builds a `RawLogRecord`
      from the request state and calls `h.rawLogger.Write(record)`
- [x] 3.4 Call `writeRawLog` at the end of `ServeHTTP` (after `persistRequest`
      and `writeUsageDebug`)
- [x] 3.5 Truncate request body to 1024 bytes, base64-encode it, set
      `request_body_truncated: true` if truncated

## 4. Tests

- [x] 4.1 Add `raw_log_test.go` with `TestOpenRawLogger` (create temp file,
      write record, verify content)
- [x] 4.2 Add `TestRawLoggerNilPath` (empty path returns nil, no panic)
- [x] 4.3 Add `TestRawLogRecordSerialization` (verify all fields appear in JSON
      output)
- [x] 4.4 Add `TestRawLogBodyTruncation` (body > 1024 bytes produces truncated
      base64 + `truncated: true`)
- [x] 4.5 Add `TestRawLogBodyNotTruncated` (body <= 1024 bytes produces full
      base64 + `truncated: false`)
- [x] 4.6 Add integration test: proxy with `--raw-log` writes records to file

## 5. Verification

- [x] 5.1 Run `just all` and confirm all tests pass
- [x] 5.2 Run `openspec validate --specs` and confirm proxy spec still validates
- [x] 5.3 Manual smoke test: `copilot-monitor run --raw-log /tmp/test.jsonl` →
      send request → verify file contains JSON record
- [x] 5.4 Manual smoke test: confirm `copilot-monitor serve --raw-log` fails
      with clear error
