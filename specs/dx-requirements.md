# Developer Experience (DX) Non-Functional Requirements

> **Audience:** Product owners, engineers, and reviewers evaluating the
> llm-proxy from a user-experience and adoption perspective.
>
> These requirements define how the system _feels_ to use — not what it _does_.
> They complement the functional requirements in `product-requirements.md`.

---

## 1. Onboarding — Time to First Dopamine

A new user should go from "never heard of llm-proxy" to "seeing their first
usage data" in under two minutes with no external documentation.

| ID      | Requirement                                                                                                                                                                                                                                           | Why it matters                                                                                                                          |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-001 | A single `llm-proxy init` command must produce a working starter route config (interactive or auto-detected) for the most common providers (OpenAI, Anthropic, GitHub Copilot) without the user reading the readme.                                   | The #1 onboarding friction is writing a correct `routes.json`. Without scaffolding, users face a JSON schema they've never seen before. |
| NFR-002 | After `llm-proxy run --routes-config starter.json`, the proxy must show a clear, actionable first-line message: a URL, a count of routes loaded, and one command the user can run next to verify it works (e.g., `curl http://127.0.0.1:7733/_ping`). | Users need immediate feedback that the thing is running and a concrete next step. A wall of log lines is intimidating.                  |
| NFR-003 | The download-and-run path (from GitHub release to first request proxied) must require at most 3 shell commands, not counting tool configuration.                                                                                                      | Every command beyond 3 is a drop-off point. The current path (download, extract, create routes.json, run, serve) is 5+.                 |
| NFR-004 | Every example route config in `examples/routes/` must work when copied verbatim with the matching provider's real API key present.                                                                                                                    | Users copy-paste example configs. If they don't work out of the box, trust is lost before the tool is even used.                        |

---

## 2. Day-to-Day Ergonomics — Common Workflows

The CLI should feel like a natural part of a developer's shell toolkit — fast,
predictable, and composable.

| ID      | Requirement                                                                                                                                                                                                                 | Why it matters                                                                                                        |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| NFR-005 | All reporting commands (`stats`, `cost`, `sessions`, `today`, `live`) must respond in under 500ms for databases with fewer than 1 million rows.                                                                             | Users query usage multiple times per day. A command that takes 2+ seconds feels broken.                               |
| NFR-006 | The `live` command (without `--watch`) must exit immediately after printing the current session; `--watch` must refresh in place every 2 seconds without flickering.                                                        | Users check "what's happening right now" frequently. A hanging command or a scrolling wall of output is unusable.     |
| NFR-007 | `llm-proxy run` and `llm-proxy serve` must be combinable into a single foreground process (`run --dashboard`) that serves proxy + dashboard on one port, so users don't need tmux or background processes for everyday use. | Managing two processes is friction. The `--dashboard` flag exists (good) but must be the documented default workflow. |
| NFR-008 | Every CLI flag that accepts a path (`--db`, `--routes-config`, `--usage-debug-log`) must expand `~` and environment variables (`$XDG_DATA_HOME`) consistently.                                                              | Users expect shell conventions to work. A config file at `~/work/proxy/routes.json` should not fail.                  |

---

## 3. Configuration UX — Route Config Format

The route config is the primary user surface. It must be easy to write, hard to
get wrong, and quick to debug when it is wrong.

| ID      | Requirement                                                                                                                                                                                                                   | Why it matters                                                                                                                                 |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-009 | Validation errors must include the route label (or path if no label) and the exact field that failed, e.g., `route "chat" (/v1/chat/completions): upstream_host is required` instead of `route 0: upstream_host is required`. | Index-based error messages force users to count JSON array entries — a terrible UX when a file has 10+ routes.                                 |
| NFR-010 | A `llm-proxy validate --routes-config path.json` command must exist, parse the file, run all validations, and exit 0 or 1 with human-readable errors on stderr.                                                               | Users should not have to start the proxy to learn their config is malformed.                                                                   |
| NFR-011 | The route config must support `//` comments or HJSON/YAML as an alternative to strict JSON, because route configs are hand-written, not machine-generated.                                                                    | Pure JSON with no comments forces users to maintain separate docs for route descriptions. This is a known pain point for hand-written configs. |
| NFR-012 | The five capture modes (`usage`, `metadata`, `none`, `tunnel`, `local`) must each have a one-sentence description in `--help` output and in the error message when an invalid value is given.                                 | Opaque string values like `"tunnel"` mean nothing to a new user. The tool must teach as it rejects.                                            |

---

## 4. Reliability & Trust — No Silent Data Loss

Users route production AI traffic through this proxy. They must trust it
implicitly.

| ID      | Requirement                                                                                                                                                                                                                                                     | Why it matters                                                                                                                                                                  |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-013 | When a route has `capture: "usage"` and the upstream returns a response without usage tokens, the request must be persisted with zero token counts and a `usage_missing` flag, not silently dropped.                                                            | The current behavior (drop requests without usage) means the user _loses data_ silently. A missing row in stats looks like the request never happened — catastrophic for trust. |
| NFR-014 | The proxy must expose a Prometheus-style metrics endpoint or a `/metrics` counter for total requests, proxied requests, blocked requests, failed upstream requests, and capture-dropped requests (with a `reason` label).                                       | Without counters, operators can't tell if the proxy is working or silently failing. This is table-stakes for any network proxy.                                                 |
| NFR-015 | Median per-request overhead (proxy latency minus upstream latency) must be under 5ms for non-streaming requests and under 1ms for streaming requests (p99 under 20ms).                                                                                          | Users will abandon the proxy if they can feel the latency. Streaming is especially sensitive because buffering destroys the user experience.                                    |
| NFR-016 | The proxy must emit a startup line listing every loaded route with its path, upstream, and capture mode, and the startup must fail (exit 1) if any route's upstream host is unreachable at bind time (optional: configurable with `--skip-connectivity-check`). | Silent misconfiguration (e.g., a typo in `upstream_host`) leads to 502 errors at runtime. A connectivity check at startup surfaces problems immediately.                        |

---

## 5. Observability — Debugging Ability

When something goes wrong, the user must be able to answer "why" within 30
seconds without reading source code.

| ID      | Requirement                                                                                                                                                                                                                                  | Why it matters                                                                                                                            |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-017 | Every proxied request must produce a single structured log line (JSON by default, optionally human-readable) containing: request ID, method, path, upstream, model, status code, latency ms, capture mode, and whether tokens were captured. | The current log lines are inconsistent and hard to parse. A single structured line per request enables `grep`, `jq`, and log aggregation. |
| NFR-018 | The proxy must inject a unique `X-LLM-Proxy-Id` header into the upstream request so the upstream's logs can be correlated with the proxy's logs.                                                                                             | Without correlation IDs, users can't tell whether a problem is in the proxy or the upstream.                                              |
| NFR-019 | The `llm-proxy run --log-format json` flag must switch all log output to newline-delimited JSON, preserving all structured fields, so users can pipe to `jq` or a log collector.                                                             | Colored terminal output is useless in production / systemd / Docker. Structured JSON is the industry standard for observability.          |
| NFR-020 | When the live tail is active (TTY) and stderr is also redirected to a file, both the live tail and the full request log must be written to their respective destinations (TTY gets live, file gets full log), not one suppressing the other. | The current behavior (either live tail OR request log) breaks the common pattern of `llm-proxy run > /var/log/proxy.log 2>&1`.            |

---

## 6. Integration Surface — Scripting and Composability

The tool must compose well with shell scripts, systemd, Docker, launchd, and CI
pipelines.

| ID      | Requirement                                                                                                                                                                                    | Why it matters                                                                                                                             |
| ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| NFR-021 | Exit codes must follow the convention: 0 = success, 1 = runtime/user error, 2 = usage error (bad flags).                                                                                       | Every automation tool (systemd, Docker HEALTHCHECK, CI scripts) relies on exit codes. Undocumented or inconsistent codes break automation. |
| NFR-022 | All environment variables that affect behavior (`NO_COLOR`, `TERM`, `XDG_DATA_HOME`, `LLM_PROXY_LISTEN`, `LLM_PROXY_DB`) must be listed in `llm-proxy help environment` or in `--help` output. | Hidden configuration surfaces are a maintenance trap. Users discover env vars by reading source code or not at all.                        |
| NFR-023 | `llm-proxy run` must support `SIGHUP` to reload the routes config without restarting the process, logging the result (success or error per route).                                             | Restarting the proxy drops in-flight requests. Config changes (adding a new provider route) should be live-reloadable.                     |
| NFR-024 | All CLI reporting commands that accept `--json` must exit 0 and write valid JSON to stdout even when there are zero results (output: `[]` or `{}`).                                            | An empty result set is not an error. Commands that exit non-zero or write nothing break pipeline chains.                                   |

---

## 7. Documentation & Examples — Persona-Driven

Every persona needs a clear, opinionated path through the docs that matches
their use case.

| ID      | Requirement                                                                                                                                                                                                                                                 | Why it matters                                                                                                                                             |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-025 | The README must contain three copy-pasteable "recipes": one for GitHub Copilot users, one for OpenAI-via-anything users, and one for Anthropic users — each starting from `llm-proxy init` and ending with a dashboard screenshot.                          | Most users read the README first and decide in 30 seconds whether the tool is for them. A wall of generic text loses them.                                 |
| NFR-026 | A `docs/troubleshooting.md` file must exist with the 5 most common errors and their solutions: "unknown route", "upstream request failed", "no usage captured", "connection refused", "no such host".                                                       | Users hit these errors within the first 5 minutes. If they have to open a GitHub issue, they may never come back.                                          |
| NFR-027 | Example route configs must include a `local.json` example (Ollama, LocalAI, LM Studio) showing `capture: "usage"` with an HTTP upstream on localhost, so locally-hosted-model users have a working reference.                                               | Local-model users are a growing persona (privacy, air-gapped, cost control). They need a concrete example, not a generic "any OpenAI-compatible API" note. |
| NFR-028 | Every example route config in `examples/routes/` must have a header comment (via a companion `README.md` in the directory) explaining each field and what the common pitfalls are (e.g., "path is case-sensitive", "Anthropic needs `prefix_match: true`"). | Raw JSON files with no explanation force users to reverse-engineer the meaning of each field.                                                              |

---

## 8. Performance — Resource Footprint

The proxy runs continuously on developer machines. It must be invisible in
resource profiles.

| ID      | Requirement                                                                                                                                                                                                | Why it matters                                                                                                                                   |
| ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| NFR-029 | The `llm-proxy run` process must use fewer than 30 MB of RSS at idle (no traffic) and fewer than 80 MB under load (100 concurrent requests).                                                               | Developers notice a 200 MB Go binary sitting in their menu bar. The proxy must feel "free" to run.                                               |
| NFR-030 | Startup time from `llm-proxy run` to "listening on" must be under 200ms on a modern machine (SSD, no cold cache).                                                                                          | A slow startup feels broken. Users restart the proxy frequently during setup.                                                                    |
| NFR-031 | Request body buffering must be limited to a configurable maximum (default 1 MB) and return 413 Payload Too Large for larger bodies, to prevent memory exhaustion from large image uploads or code context. | The current code reads the entire body into memory (`io.ReadAll`). A single large request (e.g., multi-MB code context) can OOM the proxy.       |
| NFR-032 | SQLite writes must use WAL mode and batched inserts (or a write queue) so proxy latency is not impacted by disk I/O on the same request path.                                                              | WAL mode is already enabled (good). But `InsertRequest` fires a separate `ExecContext` per request. Under load, this serializes on SQLite locks. |

---

## 9. Error Handling — Graceful Degradation

Errors happen. The proxy must surface them clearly, never lose data, and never
block the client from getting a response.

| ID      | Requirement                                                                                                                                                                                                                                                                          | Why it matters                                                                                                                                      |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| NFR-033 | An unknown path must return HTTP 404 (Not Found) instead of 502 (Bad Gateway), because the client made a bad request, not the proxy or upstream.                                                                                                                                     | Returning 502 for an unknown route is semantically wrong and confuses clients, which may retry or surface the wrong error to the user.              |
| NFR-034 | When the upstream is unreachable (DNS failure, connection refused, TLS error), the proxy must return HTTP 502 with a JSON body containing the error category (`dns`, `connection`, `tls`, `timeout`) and the upstream host, so the user can immediately tell which provider is down. | A generic "upstream request failed" message forces the user to inspect logs. Categorized errors enable automated alerting and faster debugging.     |
| NFR-035 | If the store's `InsertRequest` fails (disk full, SQLite locked), the proxy must log the failure with the request ID and continue proxying the response to the client without blocking.                                                                                               | A storage failure must not become an availability failure. Users prefer losing observability over losing their AI tool access.                      |
| NFR-036 | The proxy must expose a `GET /health` endpoint (on the proxy port) returning HTTP 200 with JSON `{"status":"ok","uptime_seconds":N,"requests_total":N,"db_size_bytes":N}` for use in Docker HEALTHCHECK and monitoring.                                                              | The current `/_ping` returns just "OK". Operators need a richer health endpoint to distinguish "process is alive" from "proxy is actually working". |

---

## Summary of Key Architectural Gaps

| NFR     | Gap                                     | Effort                     |
| ------- | --------------------------------------- | -------------------------- |
| NFR-001 | No `init` command                       | Medium                     |
| NFR-009 | Index-based error messages              | Small                      |
| NFR-013 | Silent drop of usage-less requests      | Medium (data model change) |
| NFR-015 | No latency overhead measurement         | Small (benchmark)          |
| NFR-017 | No structured JSON log output           | Small                      |
| NFR-019 | `--log-format json` flag                | Small                      |
| NFR-023 | No SIGHUP config reload                 | Medium                     |
| NFR-033 | Unknown path returns 502 instead of 404 | Small                      |
| NFR-036 | No rich `/health` endpoint              | Small                      |
