# Architecture & Onboarding

This project is a small Go CLI around two local services:

- `copilot-monitor run` starts the loopback reverse proxy that observes Copilot traffic.
- `copilot-monitor serve` starts the local reporting API and embedded dashboard over the captured SQLite data.

The entry point is `cmd/copilot-monitor/main.go`, which delegates to `internal/cli.Run`.
Normative behavior is defined in `specs/product-requirements.md` and
`specs/privacy-requirements.md`; this document maps those requirements to the
current implementation.

## Request Lifecycle

1. VS Code or another client sends Copilot traffic to the local proxy from `internal/cli/run.go`.
2. `internal/proxy.Handler.ServeHTTP` assigns a request ID, records the start time, and routes the path through `internal/proxy/router.go`.
3. `internal/proxy/forward.go` reads the request body only long enough to forward it and extract safe metadata such as `model` and `stream`.
4. The proxy builds an HTTPS upstream request to either `api.githubcopilot.com` or `copilot-proxy.githubusercontent.com`, strips hop-by-hop headers, and disables compression so response usage can be observed.
5. Responses are streamed back to the client as they arrive. For JSON and SSE responses, `internal/proxy/sse.go` watches chunks for `usage` and response `model` fields without retaining the full response.
6. `internal/proxy/server.go` persists only metadata and token counts through `internal/store.InsertRequest` when the route is configured for capture. WebSocket `/responses` traffic is tunneled by `internal/proxy/websocket.go` and is not persisted.
7. CLI reporting commands and the dashboard API read aggregate data from `internal/store`.

Capture behavior is centralized in `RoutePath`:

- `CaptureUsage`: persist only when usage tokens are found, such as chat, agents, messages, and completions.
- `CaptureMetadata`: persist request metadata without requiring usage, currently embeddings.
- `CaptureNone`, `CaptureTunnel`, and `CaptureLocal`: do not persist request rows.

## Package Map

- `internal/cli`: command parsing and user-facing commands (`run`, `serve`, `stats`, `cost`, `today`, `sessions`, `export`, `configure-vscode`).
- `internal/proxy`: routing, forwarding, SSE/JSON usage parsing, WebSocket tunneling, persistence calls, and optional usage debug logging.
- `internal/store`: SQLite initialization, schema access, inserts, stats queries, exports, and session reconstruction.
- `internal/api`: HTTP API used by the dashboard; session endpoints rebuild derived session state before responding.
- `internal/dashboard`: embedded HTML, CSS, and JavaScript dashboard that loads Petite-Vue from `unpkg` at runtime.
- `internal/cost`: converts aggregated token stats into estimated provider list-price costs.
- `internal/catalog`: embedded model pricing catalog and fallback lookup logic.
- `internal/log`: terminal log formatting.

## Persistence And Privacy Rules

The core invariant is: do not store prompts, completions, source code, auth material, cookies, or API keys.

The `requests` table stores timestamps, endpoint/path, upstream host, model, stream flag, HTTP status, latency, token counts, project label, and optional session linkage. The `sessions` table is derived from request timestamps and a 30-minute inactivity gap. `schema.sql` currently contains a `bodies` table, but production code does not write to it; do not start using it without an explicit privacy review.

Request bodies are parsed for metadata and then forwarded. Response bodies are streamed to the client while observers look for usage fields. Debug usage logs are opt-in via `--usage-debug-log` and must stay metadata-only; `SafeHeaders` redacts sensitive response headers.

Keep listeners loopback-only by default (`127.0.0.1`) unless there is a clear security reason to expose them.

## Schema Changes

The schema is embedded from `internal/store/schema.sql` and applied with `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` during `store.Open`. There is no migration framework.

When changing persisted data:

- Prefer additive columns or indexes that are safe for existing databases.
- Update `store.RequestRecord`, `InsertRequest`, relevant query structs, exports, API handlers, and CLI output together.
- Add or update `internal/store/*_test.go` coverage for insert/query behavior.
- If a change cannot be expressed safely with the current init-only schema, add a deliberate migration path before relying on it.
- Re-check privacy rules before adding any column that could contain user content, file paths, repository names, headers, or secrets.

## Common Changes

- Add or change a proxied Copilot route: start in `internal/proxy/router.go`, then update router and proxy tests.
- Change usage parsing: edit `internal/proxy/capture.go` or `internal/proxy/sse.go`, with tests beside those files.
- Change what gets persisted: edit `internal/proxy/server.go`, `internal/store/store.go`, and `internal/store/schema.sql`.
- Add a CLI report: add a command file in `internal/cli`, wire it in `internal/cli/root.go`, and query through `internal/store`.
- Add a dashboard/API metric: add or update a store query, expose it from `internal/api`, document it in `docs/api.md`, and update files in `internal/dashboard`.
- Update pricing or model normalization: edit `internal/catalog/models.json`, `internal/catalog/catalog.go`, or `internal/cost/cost.go`.
- Change session behavior: look at `internal/store/sessions.go` and the `sessions` command/API handlers.

Run `just test` for ordinary changes and `just all` before submitting broader work.

## Requirement Traceability

| Requirement Area | Primary Implementation | Test Focus |
|---|---|---|
| `PROD-002`, `ROUTE-*` supported proxy routing | `internal/proxy/router.go` | `internal/proxy/router_test.go` |
| `PROD-003`, `QUAL-001` forwarding behavior | `internal/proxy/forward.go`, `internal/proxy/server.go`, `internal/proxy/websocket.go` | `internal/proxy/*_test.go` |
| `PROD-004`, `PRIV-001` through `PRIV-005` capture and privacy boundaries | `internal/proxy/capture.go`, `internal/proxy/sse.go`, `internal/proxy/server.go`, `internal/proxy/usage_debug.go` | `internal/proxy/*_test.go` |
| `PROD-006`, `REPORT-*` CLI reports | `internal/cli/` | `internal/cli/cli_test.go` |
| `PROD-007`, `REPORT-004` read-only API and dashboard | `internal/api/`, `internal/dashboard/` | `internal/api/api_test.go` |
| `PROD-008` pricing estimates | `internal/catalog/`, `internal/cost/` | `internal/catalog/*_test.go`, `internal/cost/*_test.go` |
| `PROD-010` sessions | `internal/store/sessions.go` | `internal/store/sessions_test.go` |
| `PRIV-006` through `PRIV-010` locality, export, and sensitive derived data | `internal/store/`, `internal/cli/export.go`, `internal/api/export.go` | `internal/store/*_test.go`, `internal/cli/cli_test.go`, `internal/api/api_test.go` |
