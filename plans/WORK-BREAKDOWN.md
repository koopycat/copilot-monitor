# Work Breakdown: Phases 3–5 + DX Requirements

> **Status:** Phases 1+2 complete. This document covers all remaining work.
> **Date:** 2026-07-10 **Inputs:** `plans/universal-ai-proxy.md`,
> `specs/dx-requirements.md`, `specs/dx-review-glm52.md` **Prioritization:** GLM
> review Top 5 first, then highest-impact missing items, then original plan
> phases.

---

## Phase Ordering Rationale

1. **Phase 0 (DX Quick Wins)** — These are the adoption funnel. Without `init`,
   structured logs, and non-silent data loss, users bounce within the first 5
   minutes. Every item here is small, high-leverage, and testable in CI. Doing
   these first means every subsequent chunk benefits from a professional base.

2. **Phase 1 (Reliability & Trust)** — The `doctor` command subsumes validate,
   health, and connectivity checks into one surface. Combined with graceful
   shutdown and proper error categorization, these make operators trust the
   proxy enough to run it continuously. No one builds automation around a tool
   they don't trust.

3. **Phase 2 (DX Polish — the "feels native" layer)** — Shell completion, help
   quality, `--dry-run`, color discipline, and deterministic output. These are
   what separate "a proxy" from "a tool devs reach for daily" (the
   ripgrep/fzf/lazygit bar). Cheap individually, transformative in aggregate.

4. **Phase 3 (Advanced Routing)** — Catch-all, host-based, header manipulation,
   scheme selection. This unlocks multi-provider scenarios beyond what the
   current prefix-match router supports. Depends on Phase 0+1 being stable.

5. **Phase 4 (Dashboard, Docs & Unbranding)** — The final public-facing
   deliverables. Recipes, troubleshooting, migration guide, dashboard provider
   filter. Best done last because the CLI surface must be frozen before writing
   docs that reference it.

---

## Phase 0: DX Quick Wins

> The Top 5 must-haves from the GLM review, plus immediately adjacent cheap
> wins. Total estimated effort: ~2–3 days of focused work. Priority: **P0** —
> must-have for launch.

### Chunk 0.1 — Stop silent data loss (`usage_missing` flag)

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — existential bug; a monitoring tool that hides data is dead                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **NFRs**                | NFR-013                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| **Files**               | `internal/store/store.go` (schema migration, `RequestRecord` struct), `internal/proxy/server.go` (capture path), `internal/proxy/capture.go` (usage extraction fallback)                                                                                                                                                                                                                                                                                                                                    |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| **Acceptance Criteria** | 1. Schema has `usage_missing BOOLEAN NOT NULL DEFAULT 0` column.<br>2. When `capture: "usage"` and upstream returns no token fields, the request is persisted with zero tokens and `usage_missing = 1`.<br>3. `SELECT * FROM requests WHERE usage_missing = 1` returns those rows.<br>4. The `stats` and `cost` CLI commands show a footnote like `(N requests had no usage data)` when any exist.<br>5. Test: send a mock response with no `usage` field → assert row is created with `usage_missing = 1`. |

**Implementation sketch:**

- Add `UsageMissing bool` to `RequestRecord` struct.
- Add `usage_missing INTEGER NOT NULL DEFAULT 0` to the `CREATE TABLE` in
  `store.go:init()`.
- In `server.go`'s capture path: if `findUsage()` returns `(Usage{}, false)`,
  instead of skipping `InsertRequest`, call it with zero tokens and
  `UsageMissing: true`.
- Remove the `if usage.TotalTokens == 0 { return }` early exit that drops the
  row.
- Add a test in `server_test.go` covering both the drop case (old behavior) →
  now persist case.

---

### Chunk 0.2 — `init` command (with idempotency)

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — adoption funnel entry point                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| **NFRs**                | NFR-001, plus idempotent/no-clobber from GLM missing #7                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| **Files**               | `internal/cli/init.go` (new), `internal/cli/root.go` (register subcommand), `examples/routes/` (template sources)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| **Acceptance Criteria** | 1. `llm-proxy init` writes a starter `routes.json` to `$XDG_CONFIG_HOME/llm-proxy/routes.json` (fallback `~/.config/llm-proxy/routes.json`).<br>2. Refuses to overwrite an existing file without `--force`.<br>3. Auto-detects: if `OPENAI_API_KEY` is set → includes an OpenAI route; if `ANTHROPIC_API_KEY` → includes Anthropic; otherwise includes a generic OpenAI stub with `YOUR_API_KEY` placeholder.<br>4. Prints the file path and the next command: `llm-proxy run --routes-config <path>`.<br>5. The generated file passes `llm-proxy validate --routes-config <path>` (validates chunk 0.5). |

**Implementation sketch:**

- New file `internal/cli/init.go` with `runInit(args, stdout, stderr) int`.
- Check env vars `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` to decide which route
  stubs to include.
- Use XDG path resolution (shared helper with chunk 0.8).
- On write, check if file exists → if yes, error unless `--force`.
- Register in `root.go` switch.

---

### Chunk 0.3 — First-line startup message

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — immediate feedback after `run`                                                                                                                                                                                                                                                                                                                                                                                                    |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| **NFRs**                | NFR-002                                                                                                                                                                                                                                                                                                                                                                                                                                |
| **Files**               | `internal/cli/run.go`                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **Dependencies**        | None (best done alongside or after 0.2 so "next command" is consistent)                                                                                                                                                                                                                                                                                                                                                                |
| **Acceptance Criteria** | 1. The very first line written to stderr after `llm-proxy run` starts is: `llm-proxy: listening on 127.0.0.1:7733 (N routes loaded) → curl http://127.0.0.1:7733/_ping`.<br>2. This appears before any structured log lines.<br>3. The message includes the actual listen address, actual route count, and a concrete verification command.<br>4. Test: capture stderr for first 500ms after start, assert first line matches pattern. |

**Implementation sketch:**

- In `runServer()`, after successful `net.Listen` and config load, write the
  banner line to stderr before entering the serve loop.
- Format route count from `len(cfg.Routes)`.

---

### Chunk 0.4 — Structured JSON log line per request

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Priority**            | P0 — trust + debuggability backbone                                                                                                                                                                                                                                                                                                                                                                                                                                |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **NFRs**                | NFR-017, NFR-019 (merged per GLM review)                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **Files**               | `internal/log/log.go`, `internal/log/term.go`, `internal/proxy/server.go` (emit structured log after each request)                                                                                                                                                                                                                                                                                                                                                 |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| **Acceptance Criteria** | 1. Every proxied request produces exactly one log line on stderr containing: `request_id`, `method`, `path`, `upstream`, `model`, `status`, `latency_ms`, `capture_mode`, `tokens_captured` (bool), and optionally `usage_missing` (bool).<br>2. Default format is JSON (newline-delimited).<br>3. `--log-format human` switches to the current human-readable format.<br>4. Test: proxy a request, parse stderr line as JSON, assert all required fields present. |

**Implementation sketch:**

- Add a `RequestLog` struct in `log.go` with the required fields.
- In `server.go` after `proxyToUpstream` completes, construct a `RequestLog` and
  call `h.log.Request(logEntry)`.
- `log.go`: if JSON mode (default), marshal to JSON + newline. If human mode,
  use current format.
- Add `--log-format` flag to `run` subcommand.

---

### Chunk 0.5 — Labeled config errors

| Field                   | Detail                                                                                                                                                                                                                                                                                                                         |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Priority**            | P0 — #1 user surface is the route config                                                                                                                                                                                                                                                                                       |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                              |
| **NFRs**                | NFR-009                                                                                                                                                                                                                                                                                                                        |
| **Files**               | `internal/proxy/config.go` (`Validate()` method)                                                                                                                                                                                                                                                                               |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                           |
| **Acceptance Criteria** | 1. All validation errors include the route label (if set) or the path, e.g., `route "chat" (/v1/chat/completions): upstream_host is required` instead of `route 0: upstream_host is required`.<br>2. The field name is named in the error.<br>3. Test: load a config with errors, assert error messages contain label + field. |

**Implementation sketch:**

- Refactor the helper inside `Validate()` to build a route identifier string:
  `routeID = rc.Label + " (" + rc.Path + ")"` or fall back to `rc.Path` or
  `route index N`.
- Replace all `fmt.Errorf("route %d (...)")` calls with the new identifier.
- Minimal code change — ~15 lines modified.

---

### Chunk 0.6 — `validate` subcommand

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — fail before `run`                                                                                                                                                                                                                                                                                                                                                                                                            |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| **NFRs**                | NFR-010                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **Files**               | `internal/cli/validate.go` (new), `internal/cli/root.go`                                                                                                                                                                                                                                                                                                                                                                          |
| **Dependencies**        | Best after 0.5 (labeled errors) to show quality output                                                                                                                                                                                                                                                                                                                                                                            |
| **Acceptance Criteria** | 1. `llm-proxy validate --routes-config path.json` exits 0 on valid config, exits 1 on invalid.<br>2. Errors are printed to stderr using the labeled format from chunk 0.5.<br>3. On success, prints `routes config is valid (N routes)` to stdout.<br>4. Works with JSONC files (strips `//` comments before parsing — see 0.7).<br>5. Test in `cli_test.go`: valid config → exit 0, invalid → exit 1 with expected error string. |

**Implementation sketch:**

- New `internal/cli/validate.go` with `runValidate(args, stdout, stderr) int`.
- Call `proxy.LoadConfig(path)` and check error.
- Register in `root.go`.

---

### Chunk 0.7 — JSONC support for route configs

| Field                   | Detail                                                                                                                                                                                                                                                                                         |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — route configs are hand-written; comments are essential                                                                                                                                                                                                                                    |
| **Effort**              | S                                                                                                                                                                                                                                                                                              |
| **NFRs**                | NFR-011 (scoped to JSONC per GLM review — "cut to JSONC only")                                                                                                                                                                                                                                 |
| **Files**               | `internal/proxy/config.go`                                                                                                                                                                                                                                                                     |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                           |
| **Acceptance Criteria** | 1. Route config files can contain `// line comments` and `/* block comments */`.<br>2. Comments are stripped before JSON parsing.<br>3. All existing configs still parse correctly (backward compatible).<br>4. Example configs in `examples/routes/` are updated to include helpful comments. |

**Implementation sketch:**

- Add a `stripJSONComments(data []byte) []byte` helper in `config.go`.
- Two-line approach: strip `//.*$` per line, then strip `/* ... */` with a
  simple state machine (or use a regex for the common case).
- Call it in `LoadConfig()` before `json.Unmarshal`.

---

### Chunk 0.8 — Rich `/health` endpoint + consistent exit codes

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Priority**            | P0 — integration-enabling (Docker, systemd, CI)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| **NFRs**                | NFR-036, NFR-021                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| **Files**               | `internal/api/health.go`, `internal/cli/run.go` (SIGINT → exit 130, bad flags → exit 2)                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **Acceptance Criteria** | 1. `GET /_health` on the proxy port returns `{"status":"ok","uptime_seconds":N,"requests_total":N,"db_size_bytes":N}` with HTTP 200.<br>2. Return 503 if the store is unreachable or the DB file is missing.<br>3. `/_ping` still returns `OK` for backward compat (or redirect to `/_health`).<br>4. Exit codes: 0 = success, 1 = runtime error, 2 = usage error (bad command/flag), 130 = SIGINT.<br>5. Test: start + wait 1s + `curl /_health` → 200 with required JSON fields.<br>6. Test: `llm-proxy badcommand` exits 2. |

**Implementation sketch:**

- Expand `handleHealth` in `health.go`: compute uptime from server start time,
  `store.CountRequests()` (or a new method), `os.Stat` on DB file for size.
- Add a shared `RequestCounter` (atomic int64) in the proxy `Handler` that
  increments on each request, exposed via a getter.
- In `run.go`, add `signal.Notify(c, os.Interrupt)` handler → graceful shutdown
  → `os.Exit(130)`.
- Audit `root.go` switch: unknown commands already return 2 ✓.

---

### Phase 0 Summary

| Chunk                           | Effort        | Delivers                    |
| ------------------------------- | ------------- | --------------------------- |
| 0.1 `usage_missing` flag        | M             | No more silent data loss    |
| 0.2 `init` command              | M             | Under-1-minute onboarding   |
| 0.3 First-line message          | S             | Immediate feedback          |
| 0.4 Structured log lines        | M             | Debuggability backbone      |
| 0.5 Labeled config errors       | S             | Legible misconfig diagnosis |
| 0.6 `validate` command          | S             | Fail-fast before `run`      |
| 0.7 JSONC comments              | S             | Hand-written configs        |
| 0.8 Rich `/health` + exit codes | S             | Operator integration        |
| **Phase 0 total**               | **~2.5 days** |                             |

---

## Phase 1: Reliability & Trust

> The "trust it enough to run 24/7" layer. Builds on Phase 0's stability
> guarantees. Total: ~2 days.

### Chunk 1.1 — `doctor` command

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P0 — subsumes validate + health + connectivity into one verb                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| **NFRs**                | Derived from GLM missing #14; synthesizes NFR-010, NFR-016 (inverted to opt-in)                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| **Files**               | `internal/cli/doctor.go` (new), `internal/cli/root.go`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| **Dependencies**        | 0.5, 0.6, 0.8                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| **Acceptance Criteria** | 1. `llm-proxy doctor [--routes-config path] [--db path]` runs a sequence of checks and reports pass/fail for each.<br>2. Checks: DB writable, routes parse without errors, each upstream host reachable (TCP connect with 3s timeout, opt-in via `--check-connectivity`), disk space > 100MB available, version printed.<br>3. Exit 0 if all pass, exit 1 if any fail.<br>4. Output is a clean table: `✓ DB writable`, `✓ Routes valid (3 routes)`, `✗ Upstream api.openai.com: connection refused`, etc.<br>5. Respects `--no-color` / `NO_COLOR`. |

**Implementation sketch:**

- New `internal/cli/doctor.go`. Run each check sequentially, collect results.
- For connectivity: `net.DialTimeout("tcp", host+":443", 3*time.Second)`.
- For DB writable: open store, attempt a no-op write or check file permissions.
- For disk space: `syscall.Statfs` on the DB directory.
- Print results as aligned table with ✓/✗ prefix (or `OK`/`FAIL` when
  `--no-unicode`).

---

### Chunk 1.2 — Categorized upstream error responses

| Field                   | Detail                                                                                                                                                                      |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | --- | ------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                          |
| **Effort**              | M                                                                                                                                                                           |
| **NFRs**                | NFR-034 (extended), NFR-033, NFR-035                                                                                                                                        |
| **Files**               | `internal/proxy/server.go`, `internal/proxy/forward.go`                                                                                                                     |
| **Dependencies**        | None                                                                                                                                                                        |
| **Acceptance Criteria** | 1. Unknown path returns 404 (not 502) with `{"error":"unknown route","path":"/foo"}` body.<br>2. Upstream errors return 502 with `{"error":"upstream_error","category":"dns | connection | tls | timeout | http_status | rate_limited","upstream":"host","detail":"..."}`body.<br>3. Store failures during`InsertRequest`are logged with request ID and counted, but the response is still forwarded to the client (non-blocking).<br>4. A`capture_dropped_total` counter increments on store failure. |

**Implementation sketch:**

- In `server.go` `ServeHTTP`, change the unknown-route response from 502 to
  404 + JSON body.
- In `forward.go`, wrap upstream request error handling: inspect the error type
  (`net.DNSError`, `*net.OpError` with TLS, `context.DeadlineExceeded`,
  `*url.Error` with status code) → map to category string.
- For store non-blocking: wrap `store.InsertRequest` in a goroutine or
  check-and-continue pattern. Log failure but don't block response.
- Add `captureDropped atomic.Int64` to `Handler`.

---

### Chunk 1.3 — Graceful SIGINT shutdown

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                                                                               |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                                                                |
| **NFRs**                | GLM missing #10 (graceful SIGINT)                                                                                                                                                                                                                                                                                                                                                                                |
| **Files**               | `internal/cli/run.go`, `internal/cli/live.go`, `internal/cli/serve.go`                                                                                                                                                                                                                                                                                                                                           |
| **Dependencies**        | 0.8 (exit code 130)                                                                                                                                                                                                                                                                                                                                                                                              |
| **Acceptance Criteria** | 1. Ctrl-C during `run`/`live`/`serve` prints `shutting down (N requests in flight)`, flushes pending DB writes, and exits 130 within 2 seconds.<br>2. The HTTP server calls `Shutdown(ctx)` with a 5s timeout.<br>3. In-flight requests complete before the process exits (or get a clean 503 if they can't).<br>4. Test: start proxy, send Ctrl-C, assert exit code is 130 and stderr contains "shutting down". |

**Implementation sketch:**

- In `runServer`, set up `signal.NotifyContext(ctx, os.Interrupt)`.
- On context cancel: `srv.Shutdown(shutdownCtx)` with 5s timeout.
- Print status line to stderr.
- `os.Exit(130)` after cleanup.

---

### Chunk 1.4 — Body size limit (configurable, 10MB default)

| Field                   | Detail                                                                                                                                                                                                                                                                                                  |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                      |
| **Effort**              | S                                                                                                                                                                                                                                                                                                       |
| **NFRs**                | NFR-031 (adjusted per GLM review: 10MB default, per-route configurable)                                                                                                                                                                                                                                 |
| **Files**               | `internal/proxy/server.go`, `internal/proxy/config.go`                                                                                                                                                                                                                                                  |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                    |
| **Acceptance Criteria** | 1. Request bodies larger than 10MB (default) are rejected with 413 Payload Too Large.<br>2. Limit is configurable via `--max-body-size` flag on `run`.<br>3. Per-route override: `max_body_bytes` in `RouteConfig`.<br>4. Current `readAndRestoreBody` uses `io.LimitReader` with the configured limit. |

---

### Chunk 1.5 — `X-Request-Id` correlation header

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                 |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                     |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                      |
| **NFRs**                | NFR-018 (opt-in per GLM review due to WAF risk)                                                                                                                                                                                                                                                                                        |
| **Files**               | `internal/proxy/server.go`, `internal/proxy/forward.go`                                                                                                                                                                                                                                                                                |
| **Dependencies**        | 0.4 (structured logs use same request ID)                                                                                                                                                                                                                                                                                              |
| **Acceptance Criteria** | 1. Proxy generates a UUID per request and uses it as the `request_id` in structured logs (chunk 0.4).<br>2. Optionally injects `LLM-Proxy-Id: <uuid>` header into the upstream request, controlled by `--inject-request-id` flag (default off).<br>3. Echoes `X-Request-Id` back to the client in the response if the client sent one. |

---

### Phase 1 Summary

| Chunk                  | Effort      | Delivers                       |
| ---------------------- | ----------- | ------------------------------ |
| 1.1 `doctor` command   | M           | Single-command diagnostics     |
| 1.2 Categorized errors | M           | Operator-grade error responses |
| 1.3 Graceful SIGINT    | S           | Clean shutdown                 |
| 1.4 Body size limit    | S           | Memory safety                  |
| 1.5 Correlation header | S           | Debuggability                  |
| **Phase 1 total**      | **~2 days** |                                |

---

## Phase 2: DX Polish — "Feels Native"

> The missing items from the GLM review that elevate the tool from "functional"
> to "a joy to use." Total: ~2–3 days.

### Chunk 2.1 — Real per-subcommand `--help` with examples

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                        |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                                                            |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                             |
| **NFRs**                | GLM missing #1, #2                                                                                                                                                                                                                                                                                                                                                                            |
| **Files**               | `internal/cli/root.go` (refactor flag parsing), each subcommand file                                                                                                                                                                                                                                                                                                                          |
| **Dependencies**        | None (can start in parallel with Phase 1)                                                                                                                                                                                                                                                                                                                                                     |
| **Acceptance Criteria** | 1. `llm-proxy --help` is ≤ 24 lines, ends with `Run 'llm-proxy help <subcommand>' for more information.`<br>2. `llm-proxy run --help` shows: all flags with defaults, one example invocation per flag, capture-mode descriptions per NFR-012.<br>3. Same for `stats --help`, `cost --help`, `doctor --help`, etc.<br>4. Each help text ends with a `Examples:` block showing 2–3 invocations. |

**Implementation sketch:**

- Add per-command help functions: `printRunHelp(w)`, `printStatsHelp(w)`, etc.
- In each `run*()` function, check for `--help`/`-h` flag before parsing.
- Or: refactor to use a lightweight command registration struct with a `Help`
  field.

---

### Chunk 2.2 — Shell completion (bash/zsh/fish)

| Field                   | Detail                                                                                                                                                                                                            |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                |
| **Effort**              | M                                                                                                                                                                                                                 |
| **NFRs**                | GLM missing #4                                                                                                                                                                                                    |
| **Files**               | `internal/cli/completion.go` (new), `internal/cli/root.go`                                                                                                                                                        |
| **Dependencies**        | None                                                                                                                                                                                                              |
| **Acceptance Criteria** | 1. `llm-proxy completion bash` outputs a bash completion script to stdout.<br>2. Same for `zsh` and `fish`.<br>3. Completions include: subcommands, flags (with values where applicable, e.g., `--log-format json | human`), and `--routes-config` (with file completion).<br>4. Test: script is non-empty and evaluable without errors. |

**Implementation sketch:**

- Use `github.com/posener/complete` or hand-roll a simple completion generator.
- Since the CLI is hand-rolled (not cobra), write a
  `printCompletion(shell string)` function.
- For bash: generate a `_llm_proxy()` function with `COMPREPLY` logic.
- Output to stdout so user can `eval "$(llm-proxy completion bash)"`.

---

### Chunk 2.3 — `--dry-run` for `run`

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                        |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                                            |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                             |
| **NFRs**                | GLM missing #8                                                                                                                                                                                                                                                                                                                                                                |
| **Files**               | `internal/cli/run.go`                                                                                                                                                                                                                                                                                                                                                         |
| **Dependencies**        | 0.5, 0.6 (show validated config)                                                                                                                                                                                                                                                                                                                                              |
| **Acceptance Criteria** | 1. `llm-proxy run --dry-run --routes-config path.json` loads and validates the config, prints each route with its resolved upstream, capture mode, and models, then exits 0 without binding a port.<br>2. Output format is human-readable (table) or JSON with `--json`.<br>3. Test: `--dry-run` exits 0 with valid config, exits 1 with invalid config (same as `validate`). |

---

### Chunk 2.4 — `--no-color`, `NO_COLOR`, `--no-unicode` discipline

| Field                   | Detail                                                                |
| ----------------------- | --------------------------------------------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                    |
| **Effort**              | S                                                                     |
| **NFRs**                | GLM missing #5, #9 (deterministic output)                             |
| **Files**               | `internal/log/term.go`, all CLI output files (`cli/*.go`)             |
| **Dependencies**        | None                                                                  |
| **Acceptance Criteria** | 1. All colored output respects `NO_COLOR` env var and `--color=always | auto | never`flag.<br>2. Auto-detect: color is on when stderr is a TTY, off when piped.<br>3.`--no-unicode`flag replaces ✓/✗ with OK/FAIL and box-drawing chars with ASCII.<br>4. Test: set`NO_COLOR=1`, assert no ANSI escape codes in output. |

**Implementation sketch:**

- Add a shared `OutputConfig` in `cli/shared.go` with `Color`, `Unicode` bools.
- `term.go`: add `ShouldColor(w io.Writer) bool` (checks TTY + `NO_COLOR`).
- Grep all `fmt.Fprintf` calls for color escape sequences → use the config.

---

### Chunk 2.5 — Empty `--json` exits 0

| Field                   | Detail                                                                                                                                                                                                                                   |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                       |
| **Effort**              | S                                                                                                                                                                                                                                        |
| **NFRs**                | NFR-024                                                                                                                                                                                                                                  |
| **Files**               | `internal/cli/stats.go`, `internal/cli/cost.go`, `internal/cli/sessions.go`, `internal/cli/today.go`, `internal/cli/export.go`                                                                                                           |
| **Dependencies**        | None                                                                                                                                                                                                                                     |
| **Acceptance Criteria** | 1. All reporting commands with `--json` output valid JSON (`[]` or `{}`) even with zero results.<br>2. Exit code is always 0 for empty results (not an error).<br>3. Test: empty DB → `llm-proxy stats --json` outputs `[]` and exits 0. |

---

### Chunk 2.6 — Environment variable documentation

| Field                   | Detail                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                          |
| **Effort**              | S                                                                                                                                                                                                                                                                                                           |
| **NFRs**                | NFR-022                                                                                                                                                                                                                                                                                                     |
| **Files**               | `internal/cli/root.go` (add `help environment`), docs                                                                                                                                                                                                                                                       |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                        |
| **Acceptance Criteria** | 1. `llm-proxy help environment` lists every env var with description: `NO_COLOR`, `TERM`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `LLM_PROXY_LISTEN`, `LLM_PROXY_DB`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.<br>2. Output is a formatted table.<br>3. Same env vars are cross-referenced in `--help` text. |

---

### Chunk 2.7 — Expand `~` and env vars in path flags

| Field                   | Detail                                                                                                                                                                                                        |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                            |
| **Effort**              | S                                                                                                                                                                                                             |
| **NFRs**                | NFR-008                                                                                                                                                                                                       |
| **Files**               | `internal/cli/shared.go` (new helper), `internal/cli/run.go`, `internal/cli/serve.go`, `internal/cli/validate.go`                                                                                             |
| **Dependencies**        | None                                                                                                                                                                                                          |
| **Acceptance Criteria** | 1. `--db ~/data/proxy.db` expands to `$HOME/data/proxy.db`.<br>2. `--routes-config $CONFIG_DIR/routes.json` expands the env var.<br>3. Test: set env var, use `$VAR` in flag, assert correct path resolution. |

**Implementation sketch:**

- Add `expandPath(p string) string` in `cli/shared.go`: handle `~` prefix →
  `os.UserHomeDir()`, then `os.ExpandEnv`.
- Call at the start of each `run*` function for every path flag.

---

### Phase 2 Summary

| Chunk                         | Effort        | Delivers                |
| ----------------------------- | ------------- | ----------------------- |
| 2.1 Per-subcommand `--help`   | M             | Professional help UX    |
| 2.2 Shell completion          | M             | Native shell feel       |
| 2.3 `--dry-run`               | S             | Confidence before `run` |
| 2.4 Color/NO_COLOR discipline | S             | Professional output     |
| 2.5 Empty `--json` exits 0    | S             | Pipeline composability  |
| 2.6 Env var docs              | S             | No hidden config        |
| 2.7 Path expansion            | S             | Shell conventions       |
| **Phase 2 total**             | **~2.5 days** |                         |

---

## Phase 3: Advanced Routing

> Unlocks multi-provider scenarios. Depends on Phases 0+1 for a stable base.
> Total: ~3 days.

### Chunk 3.1 — Catch-all route (`path: "/*"`)

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                         |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                          |
| **NFRs**                | Original plan Phase 3, item 2                                                                                                                                                                                                                                                                                                                              |
| **Files**               | `internal/proxy/router.go`, `internal/proxy/config.go`, `internal/proxy/server.go`                                                                                                                                                                                                                                                                         |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                       |
| **Acceptance criteria** | 1. A route with `path: "/*"` matches any request path not matched by more specific routes.<br>2. More specific routes (exact or prefix) always win over the catch-all.<br>3. The catch-all route supports all capture modes.<br>4. Test: send requests to `/v1/chat/completions`, `/random/path`, `/` — all match the catch-all if no other route matches. |

---

### Chunk 3.2 — Host-based routing

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                     |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                                                         |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                          |
| **NFRs**                | Original plan Phase 3, item 1                                                                                                                                                                                                                                                                                                                                                              |
| **Files**               | `internal/proxy/router.go`, `internal/proxy/config.go`, `internal/proxy/server.go`                                                                                                                                                                                                                                                                                                         |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                                                                                                       |
| **Acceptance Criteria** | 1. `RouteConfig` gains an optional `host` field.<br>2. Routes with `host` set only match requests whose `Host` header matches.<br>3. Routes without `host` match any host (current behavior).<br>4. Host + path matching: host is a secondary filter after path.<br>5. Test: two routes with different `host` values → requests are dispatched to correct upstream based on `Host` header. |

---

### Chunk 3.3 — Per-route header manipulation

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                                                                                                    |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                     |
| **NFRs**                | Original plan Phase 3, item 3                                                                                                                                                                                                                                                                                                         |
| **Files**               | `internal/proxy/config.go`, `internal/proxy/forward.go`                                                                                                                                                                                                                                                                               |
| **Dependencies**        | 3.1 or 3.2 (need the advanced route matching to be stable)                                                                                                                                                                                                                                                                            |
| **Acceptance Criteria** | 1. `RouteConfig` gains `set_headers: {"X-Custom": "value"}` and `remove_headers: ["X-Remove"]`.<br>2. Before forwarding to upstream, headers are modified per the route config.<br>3. `Authorization` header can be set per-route (enables API key injection from config).<br>4. Test: verify upstream receives the modified headers. |

---

### Chunk 3.4 — Upstream scheme selection (`http` vs `https`)

| Field                   | Detail                                                                                                                                                                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                  |
| **Effort**              | S                                                                                                                                                                                                                                                   |
| **NFRs**                | Original plan Phase 3, item 4                                                                                                                                                                                                                       |
| **Files**               | `internal/proxy/config.go`, `internal/proxy/forward.go`                                                                                                                                                                                             |
| **Dependencies**        | None                                                                                                                                                                                                                                                |
| **Acceptance Criteria** | 1. `RouteConfig` gains optional `scheme` field (default `"https"`).<br>2. `scheme: "http"` uses plain HTTP for the upstream connection.<br>3. Test: route with `scheme: "http"` and `upstream_host: "localhost:11434"` (Ollama) forwards correctly. |

---

### Chunk 3.5 — Path rewriting

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                                                                                                                                                                                               |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                                                                                                                                |
| **NFRs**                | Original plan Phase 3, item 5                                                                                                                                                                                                                                                                                                                                                                                                    |
| **Files**               | `internal/proxy/config.go`, `internal/proxy/forward.go`                                                                                                                                                                                                                                                                                                                                                                          |
| **Dependencies**        | 3.1 (catch-all)                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **Acceptance Criteria** | 1. `RouteConfig` gains optional `rewrite_path` field: a template string like `/api/v1{{$1}}` or a prefix strip + prepend model.<br>2. Simple form: `strip_prefix` removes the route's path prefix before forwarding, optionally combined with `upstream_path_prefix`.<br>3. Test: route with `path: "/legacy/*"` and `strip_prefix: "/legacy"` forwards `/legacy/chat` to upstream as `/chat` (plus any `upstream_path_prefix`). |

---

### Chunk 3.6 — Generic WebSocket route declaration

| Field                   | Detail                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                                                                          |
| **Effort**              | M                                                                                                                                                                                                                                                                                                           |
| **NFRs**                | Original plan Phase 3, item 7                                                                                                                                                                                                                                                                               |
| **Files**               | `internal/proxy/router.go`, `internal/proxy/config.go`, `internal/proxy/websocket.go`                                                                                                                                                                                                                       |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                        |
| **Acceptance Criteria** | 1. WebSocket upgrade is supported for any route that has `capture: "tunnel"` (tunnel mode passes everything through).<br>2. Remove the hardcoded check for Copilot `/responses` path in `websocket.go`.<br>3. Test: declare a WebSocket route in config, connect via `wscat`, verify bidirectional traffic. |

---

### Phase 3 Summary

| Chunk                   | Effort      | Delivers                         |
| ----------------------- | ----------- | -------------------------------- |
| 3.1 Catch-all route     | M           | Transparent forwarding           |
| 3.2 Host-based routing  | M           | Multi-tenant / vhost setups      |
| 3.3 Header manipulation | M           | API key injection, customization |
| 3.4 Scheme selection    | S           | Local backends (Ollama)          |
| 3.5 Path rewriting      | M           | Legacy path adaptation           |
| 3.6 Generic WebSocket   | M           | Non-Copilot WebSocket providers  |
| **Phase 3 total**       | **~3 days** |                                  |

---

## Phase 4: Dashboard, Docs & Unbranding

> The final public-facing deliverables. Best done after CLI surface is frozen.
> Total: ~2–3 days.

### Chunk 4.1 — Dashboard provider filter

| Field                   | Detail                                                                                                                                                                                                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                         |
| **Effort**              | M                                                                                                                                                                                                                                                          |
| **NFRs**                | Original plan Phase 4, item 5                                                                                                                                                                                                                              |
| **Files**               | `internal/api/stats.go`, `internal/api/cost.go`, `internal/api/sessions.go`, `dashboard/` (HTML/JS)                                                                                                                                                        |
| **Dependencies**        | Phase 2 complete (provider field populated from Phase 1+2)                                                                                                                                                                                                 |
| **Acceptance Criteria** | 1. API endpoints accept `?provider=openai` query parameter.<br>2. Dashboard shows a provider dropdown filter next to existing filters.<br>3. `SELECT DISTINCT provider` populates the dropdown.<br>4. Test: filter by provider returns only matching rows. |

---

### Chunk 4.2 — Rewrite README for universal proxy

| Field                   | Detail                                                                                                                                                                                                                                                                                                                 |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                     |
| **Effort**              | M                                                                                                                                                                                                                                                                                                                      |
| **NFRs**                | Original plan Phase 5; NFR-025 (three copy-pasteable recipes)                                                                                                                                                                                                                                                          |
| **Files**               | `README.md`                                                                                                                                                                                                                                                                                                            |
| **Dependencies**        | Phase 0+1 stable (commands referenced in README must work)                                                                                                                                                                                                                                                             |
| **Acceptance Criteria** | 1. README leads with "universal AI proxy" positioning.<br>2. Three recipes: GitHub Copilot, OpenAI, Anthropic — each from `llm-proxy init` to `curl` returning real JSON.<br>3. No Copilot branding in generic text; only in the Copilot recipe.<br>4. Links to `docs/troubleshooting.md` and `docs/custom-routes.md`. |

---

### Chunk 4.3 — `docs/troubleshooting.md`

| Field                   | Detail                                                                                                                                                                                                                                                                                                                                                                                      |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P1                                                                                                                                                                                                                                                                                                                                                                                          |
| **Effort**              | S                                                                                                                                                                                                                                                                                                                                                                                           |
| **NFRs**                | NFR-026                                                                                                                                                                                                                                                                                                                                                                                     |
| **Files**               | `docs/troubleshooting.md` (new)                                                                                                                                                                                                                                                                                                                                                             |
| **Dependencies**        | Phase 1 (error categories from 1.2 feed into the doc)                                                                                                                                                                                                                                                                                                                                       |
| **Acceptance Criteria** | 1. Covers the 5 most common errors: "unknown route", "upstream request failed", "no usage captured", "connection refused", "no such host".<br>2. Each error has: what it looks like, why it happened, how to fix it.<br>3. Each error string in the code includes a reference: `see docs/troubleshooting.md#unknown-route`.<br>4. Includes `llm-proxy doctor` as the first diagnostic step. |

---

### Chunk 4.4 — `docs/custom-routes.md`

| Field                   | Detail                                                                                                                                                                                                                                                                                |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                                                    |
| **Effort**              | S                                                                                                                                                                                                                                                                                     |
| **NFRs**                | Original plan Phase 5, NFR-027, NFR-028                                                                                                                                                                                                                                               |
| **Files**               | `docs/custom-routes.md` (new), `examples/routes/README.md`                                                                                                                                                                                                                            |
| **Dependencies**        | Phase 3 (advanced routing features)                                                                                                                                                                                                                                                   |
| **Acceptance Criteria** | 1. Explains every `RouteConfig` field with examples.<br>2. Includes examples for OpenAI, Anthropic, GitHub Copilot, and local backends (Ollama/LM Studio).<br>3. `examples/routes/README.md` explains common pitfalls (case-sensitive paths, `prefix_match`, capture mode semantics). |

---

### Chunk 4.5 — `docs/migration-from-copilot-monitor.md`

| Field                   | Detail                                                                                                                                                                                                                                   |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                       |
| **Effort**              | S                                                                                                                                                                                                                                        |
| **NFRs**                | Original plan Phase 5                                                                                                                                                                                                                    |
| **Files**               | `docs/migration-from-copilot-monitor.md` (new)                                                                                                                                                                                           |
| **Dependencies**        | All previous phases (migration is the last doc)                                                                                                                                                                                          |
| **Acceptance Criteria** | 1. Step-by-step: install new binary, copy example Copilot config, update scripts/systemd/VSCode settings.<br>2. Lists every breaking change from the original plan's summary.<br>3. Includes a before/after comparison of config format. |

---

### Chunk 4.6 — CLI and API unbranding audit

| Field                   | Detail                                                                                                                                                                                                                                                                                                     |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Priority**            | P2                                                                                                                                                                                                                                                                                                         |
| **Effort**              | S                                                                                                                                                                                                                                                                                                          |
| **NFRs**                | Original plan Phase 4, items 1–4, 6–7                                                                                                                                                                                                                                                                      |
| **Files**               | `dashboard/` (HTML/JS/Go embed), `internal/cli/*.go`, `internal/api/*.go`, `specs/product-requirements.md`, `AGENTS.md`                                                                                                                                                                                    |
| **Dependencies**        | None                                                                                                                                                                                                                                                                                                       |
| **Acceptance Criteria** | 1. Zero occurrences of "Copilot" in dashboard UI text (except as a provider example).<br>2. CLI help text uses "provider" and "upstream" consistently.<br>3. API docs say "LLM usage" not "Copilot usage".<br>4. `specs/product-requirements.md` rewritten for universal scope.<br>5. `AGENTS.md` updated. |

---

### Phase 4 Summary

| Chunk                         | Effort        | Delivers                      |
| ----------------------------- | ------------- | ----------------------------- |
| 4.1 Dashboard provider filter | M             | Multi-provider visibility     |
| 4.2 README rewrite            | M             | 30-second adoption decision   |
| 4.3 Troubleshooting doc       | S             | Self-service error recovery   |
| 4.4 Custom routes doc         | S             | User can write any config     |
| 4.5 Migration guide           | S             | Existing users can transition |
| 4.6 Unbranding audit          | S             | Clean, professional identity  |
| **Phase 4 total**             | **~2.5 days** |                               |

---

## Dependency Graph

```
Phase 0 (DX Quick Wins)
├── 0.1 usage_missing flag ─────────────┐
├── 0.2 init command                    │
├── 0.3 first-line message              │
├── 0.4 structured log lines ───────┐   │
├── 0.5 labeled config errors ──┐   │   │
├── 0.6 validate command ───────┤   │   │
├── 0.7 JSONC comments          │   │   │
└── 0.8 /health + exit codes ───┤───┤───┤
                                 │   │   │
Phase 1 (Reliability & Trust)    │   │   │
├── 1.1 doctor command ←────────┘───┘───┘
├── 1.2 categorized errors
├── 1.3 graceful SIGINT ←──────── (0.8)
├── 1.4 body size limit
└── 1.5 correlation header ←──── (0.4)

Phase 2 (DX Polish)
├── 2.1 per-subcommand --help
├── 2.2 shell completion
├── 2.3 --dry-run ←───────────── (0.5, 0.6)
├── 2.4 color/NO_COLOR
├── 2.5 empty --json exits 0
├── 2.6 env var docs
└── 2.7 path expansion

Phase 3 (Advanced Routing)
├── 3.1 catch-all route
├── 3.2 host-based routing
├── 3.3 header manipulation ←─── (3.1)
├── 3.4 scheme selection
├── 3.5 path rewriting ←──────── (3.1)
└── 3.6 generic websocket

Phase 4 (Dashboard, Docs & Unbranding)
├── 4.1 dashboard provider filter
├── 4.2 README rewrite ←──────── (Phases 0-1)
├── 4.3 troubleshooting doc ←──── (1.2)
├── 4.4 custom routes doc ←────── (Phase 3)
├── 4.5 migration guide ←──────── (all phases)
└── 4.6 unbranding audit
```

---

## Effort Summary

| Phase                        | Chunks | S      | M      | L     | Total Est.     |
| ---------------------------- | ------ | ------ | ------ | ----- | -------------- |
| Phase 0: DX Quick Wins       | 8      | 5      | 3      | 0     | ~2.5 days      |
| Phase 1: Reliability & Trust | 5      | 3      | 2      | 0     | ~2 days        |
| Phase 2: DX Polish           | 7      | 5      | 2      | 0     | ~2.5 days      |
| Phase 3: Advanced Routing    | 6      | 1      | 5      | 0     | ~3 days        |
| Phase 4: Docs & Unbranding   | 6      | 4      | 2      | 0     | ~2.5 days      |
| **Grand Total**              | **32** | **18** | **14** | **0** | **~12.5 days** |

> Note: Sizes are for one developer in focused "one sitting" chunks. S ≈ 2–4
> hours, M ≈ 4–8 hours. No chunk should span more than one working day.

---

## What Was Cut or Deferred

Per the GLM review's recommendations — these are explicitly _not_ in the plan:

| Item                                                   | Verdict                | Rationale                                                                                              |
| ------------------------------------------------------ | ---------------------- | ------------------------------------------------------------------------------------------------------ |
| NFR-011 HJSON/YAML                                     | **Cut to JSONC**       | One grammar is enough. `//` comments cover 95% of pain.                                                |
| NFR-014 Prometheus `/metrics`                          | **Deferred**           | `/health` counters cover 80%. Add when an operator asks.                                               |
| NFR-023 SIGHUP reload                                  | **Deferred to v2**     | Real subsystem; restart is fine for v1.                                                                |
| NFR-016 startup connectivity check (default on)        | **Inverted**           | Opt-in via `doctor --check-connectivity`, not default. Failing to start on flaky upstreams is hostile. |
| NFR-031 1MB body cap                                   | **Raised to 10MB**     | 1MB breaks real Copilot/vision traffic.                                                                |
| NFR-007 `--dashboard` as mandatory default             | **Softened**           | Documented recommended mode, not forced.                                                               |
| NFR-020 TTY + file dual output                         | **Demoted to bug fix** | Handle as part of 0.4 log restructuring.                                                               |
| NFR-015 latency overhead benchmark                     | **Deferred**           | Needs benchmark harness; not adoption-blocking.                                                        |
| GLM missing #6 machine-readable `--help --json`        | **Deferred**           | Low adoption impact for a personal proxy.                                                              |
| GLM missing #11 `llm-proxy version --check-for-update` | **Deferred**           | Can be added post-launch.                                                                              |
| GLM missing #12 static binary guarantee                | **Already met**        | Go default; no work needed.                                                                            |
| GLM missing #13 self-telemetry of errors               | **Deferred**           | Needs a real telemetry pipeline.                                                                       |
| GLM missing #15 streaming-first body handling          | **Deferred**           | Architectural change; defer until body-limit pain is real. Phase 1.4 (10MB limit) is a stopgap.        |

---

## Implementation Order (Sprint View)

If executing sequentially, the recommended order within each phase:

**Sprint 1 — Phase 0 (days 1–3):**

1. `0.5` → `0.7` → `0.6` → `0.8` (quick wins, build confidence)
2. `0.4` → `0.3` (structured logs + startup message)
3. `0.1` (schema change, most impactful but most risky — do it with a stable
   base)
4. `0.2` (init command — last, because it generates configs that depend on
   0.7/0.5)

**Sprint 2 — Phase 1 (days 4–5):**

1. `1.2` (categorized errors — immediate user-visible improvement)
2. `1.5` → `1.3` (correlation header + graceful shutdown)
3. `1.4` (body limit)
4. `1.1` (doctor command — tie it all together)

**Sprint 3 — Phase 2 (days 6–8):**

1. `2.4` → `2.7` (output discipline + path expansion — foundation)
2. `2.5` → `2.6` (pipe-friendly outputs)
3. `2.1` (per-subcommand help)
4. `2.3` → `2.2` (dry-run + completion)

**Sprint 4 — Phase 3 (days 9–11):**

1. `3.4` (scheme selection — simplest, useful for local backends)
2. `3.1` (catch-all)
3. `3.2` (host-based)
4. `3.6` (WebSocket)
5. `3.5` → `3.3` (path rewriting → header manipulation)

**Sprint 5 — Phase 4 (days 12–14):**

1. `4.6` (unbranding audit)
2. `4.1` (dashboard filter)
3. `4.3` (troubleshooting)
4. `4.4` (custom routes doc)
5. `4.2` (README rewrite)
6. `4.5` (migration guide — last, references everything)
