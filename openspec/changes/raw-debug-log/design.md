## Context

The proxy already has two logging mechanisms: structured request logs
(stdout/stderr via `internal/log`) and an optional usage debug log
(`--usage-debug-log`, via `internal/proxy/usage_debug.go`). Both are
metadata-only. The usage debug logger already provides a JSONL file writer
pattern with a mutex, `Open*`/`Close`/`Write` interface, and header redaction
via `SafeHeaders`. This change adds a third, opt-in raw logging channel that
includes request body bytes (truncated) for routing and policy debugging.

## Goals / Non-Goals

**Goals:**

- Add a `--raw-log <path>` flag to `run`
- Write one JSON record per proxied request to the file
- Include request body bytes truncated at 1024 bytes, base64-encoded
- Include redacted response headers
- Include routing, policy, and compression decisions
- Print a startup privacy warning when enabled

**Non-Goals:**

- Raw logging for WebSocket frames or SSE streams
- Full request body storage (the 1024-byte truncation is a privacy safeguard,
  not a configuration knob)
- Dashboard UI for raw logs
- Raw log in standalone `serve` mode (no proxy requests to log)

## Decisions

**Decision 1: Follow `UsageDebugLogger` pattern.** Create a `RawLogger` struct
in a new file `internal/proxy/raw_log.go` that mirrors `usage_debug.go`:
`OpenRawLogger(path)`, `Close()`, `Write(record)` with a mutex. The handler
calls `Write()` at the end of `ServeHTTP` (same point as `writeUsageDebug`).
Rationale: proven pattern, reuses `SafeHeaders`, no new abstractions.

**Decision 2: Request body truncated at 1024 bytes, stored as base64.** Raw
bytes may contain binary or multi-byte sequences. Base64 encoding avoids JSON
encoding issues and makes the output grep-safe. 1024 bytes is enough to identify
the model and first few messages without capturing entire source files.
Rationale: balance debug usefulness against privacy risk. No configuration knob
needed — users who need more can use external tools.

**Decision 3: New `RawLogRecord` struct, not reuse of `UsageDebugRecord`.** The
raw log carries different fields (request body, routing decisions, policy
decision, compression status) not present in the usage debug record. Keeping
separate structs avoids leaking debug fields into the usage log and vice versa.

**Decision 4: Flag validation in `runServer` only.** `--raw-log` is defined in
the `run` flag set; it does not appear in `serve` flags. If passed to `serve`,
Go's `flag` package will reject it as unknown. No extra validation needed.

## Risks / Trade-offs

- **[Privacy] Raw request bodies may contain source code or prompts.** →
  Mitigation: startup warning printed to stderr; 1024-byte truncation; opt-in
  only; file written with 0600 permissions.
- **[Performance] Mutex contention on the log writer.** → Mitigation: JSON
  encoding is fast; mutex scope is per-write only; same pattern as
  `UsageDebugLogger` which has not shown issues.
- **[Disk space] Unbounded log file growth.** → Mitigation: log rotation is out
  of scope for v1; documented as a caveat. Users can manage rotation externally
  (`logrotate`, `mv` + `SIGHUP`).
