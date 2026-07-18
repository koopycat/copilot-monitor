# Architecture & Onboarding

Copilot Monitor is a small Go CLI built around two local services that can run
separately or together in one process.

- `copilot-monitor run` starts a loopback reverse proxy that observes LLM API
  traffic (default `127.0.0.1:7733`).
- `copilot-monitor serve` starts the local reporting API with an embedded
  dashboard over captured SQLite data (default `127.0.0.1:7734`).
- `copilot-monitor run --dashboard` starts both proxy and dashboard in a single
  process, listening on their respective default ports.

The entry point is `cmd/copilot-monitor/main.go`, which delegates to
`internal/cli.Run`. Normative behavior is defined by the specs in
`openspec/specs/`.

## Request Lifecycle

1. A client (VS Code, pi agent, curl, etc.) sends API traffic to the local proxy
   started by `internal/cli/run.go`.
2. On startup, `run.go` validates the required `--upstream` flag and sets the
   `--headroom-proxy-addr` (default `127.0.0.1:8787`) for detecting
   headroom-compressed traffic via RemoteAddr.
3. `internal/proxy.Handler.ServeHTTP` assigns a request ID, records the start
   time, and reads the request body to extract safe metadata such as `model` and
   `stream`.
4. Built-in endpoints `/_ping` and `/_health` are handled locally without
   forwarding to the upstream.
5. `internal/policy.Policy.Allow` evaluates the global model policy. When a
   request model matches a configured blocklist, the proxy returns HTTP 403 with
   a JSON error body, persists the blocked attempt, and never contacts the
   upstream.
6. The proxy builds an HTTPS upstream request using the single `--upstream`
   host. Strip hop-by-hop headers and disable compression so response usage can
   be observed. All requests (except `/_ping` and `/_health`) are forwarded to
   the configured upstream as-is.
7. Responses are streamed back to the client as they arrive. For JSON and SSE
   responses, `internal/proxy/sse.go` watches chunks for `usage` and response
   `model` fields without retaining the full response.
8. `internal/proxy/server.go` classifies each request as `inference` or
   `control_plane` based on its endpoint path, the presence of a model, and
   whether token usage was captured, then persists metadata and token counts
   through `internal/store.InsertRequest`. WebSocket traffic (Copilot
   `/responses`) is inspected frame-by-frame by `internal/proxy/websocket.go`
   for `response.create` (model) and `response.completed` (usage) events, and
   persisted the same way as HTTP completions.
9. CLI reporting commands and the dashboard API read aggregate data from
   `internal/store`.

### Optional: Headroom compression proxy

Compression is no longer handled inline. An optional Headroom compression proxy
can run in front of Copilot Monitor for token reduction. This is not required --
the proxy functions identically without it. See the
[pipeline design doc](./proxy-pipeline-design.md) for the architecture.

When Headroom is running, requests from its address are detected via RemoteAddr
and flagged with `headroom_proxied = true`.

- `internal/catalog`: embedded model pricing catalog and fallback lookup logic.
- `internal/log`: terminal log formatting.

### Policy Enforcement

The proxy evaluates a global model policy between request body parsing and
upstream forwarding. The policy is stored in a single-row `policies` table.

- **Modes**: `allow_all` (default), `blocklist` (models listed are blocked),
  `allowlist` (only listed models pass)
- **Patterns**: model names support `*` suffix for prefix matching (`gpt-*`
  blocks all GPT models)
- **Cache**: in-memory cache with 5-second TTL, refreshed from SQLite on expiry
- **Model discovery**: successful OpenAI-compatible `GET /models` responses are
  filtered before return - an allowlist exposes only matching model IDs and a
  blocklist omits matching IDs. Client-side model configuration cannot expand
  this set.
- **Fail-open**: nil policy, empty model, unknown mode, store errors, and model
  discovery payloads that cannot safely be filtered all preserve the existing
  permissive behaviour
- **Persistence**: blocked attempts are stored in the `requests` table with
  status 403 and zero token counts

- **WebSocket policy**: after the upgrade, complete client text messages that
  explicitly name a model are checked before forwarding. A disallowed message is
  withheld, recorded as status 403, and receives a close frame with code 1008.
  Messages with no usable model, invalid JSON, or an over-limit inspection
  payload retain the documented fail-open behavior.

Policy management is through the dashboard API: `GET/PUT /api/policy` and model
discovery via `GET /api/policy/models`.

## Persistence and Privacy Rules

The core invariant is: do not store prompts, completions, source code, auth
material, cookies, or API keys. The `requests` table stores timestamps, endpoint
and path, endpoint kind (`inference` or `control_plane`), upstream host, model,
stream flag, HTTP status, latency, token counts, project label, and an optional
session link. The endpoint kind is derived at capture time; usage views
(`stats`, `cost`, `today`, `timeline`) include only `inference` traffic, while
CSV export retains every captured row with its kind. The `sessions` table is
maintained at request insertion time using a 30-minute inactivity gap and can be
rebuilt with the offline `rebuild-sessions` command. Request bodies are parsed
for metadata and then forwarded, but are never persisted. Response bodies are
streamed to the client while observers look only for usage fields. Debug usage
logs are opt-in via `--usage-debug-log` and must stay metadata-only;
`SafeHeaders` redacts sensitive response headers. Keep listeners loopback-only
by default (`127.0.0.1`) unless there is a clear security reason to expose them.

Headroom is a separate local process and may retain original content according
to its own configuration.

### Retention

`run` and `serve` prune captured requests and sessions older than 365 days on
startup and every 24 hours by default. `--retention-days 0` disables request
pruning; anomalies use their independent 30-day default, configurable (or
disabled) with `--anomaly-retention-days`. Deletions are committed in small
batches and sessions crossing a cutoff remain intact. Use `--dry-run` to report
eligible rows before deleting them. Retention never runs `VACUUM`; use
`rebuild-sessions --vacuum` for explicit compaction.

## Schema Changes

The schema is embedded from `internal/store/schema.sql` and applied with
`CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` during
`store.Open`. A migration step runs after schema init, executing
`ALTER TABLE ADD COLUMN` statements that are tolerant of duplicate-column
errors. When changing persisted data:

- Prefer additive columns and indexes that are safe for existing databases.
- Add corresponding `ALTER TABLE` statements to `runMigrations` in `store.go`.
- Update `store.RequestRecord`, `InsertRequest`, the relevant query structs,
  exports, API handlers, and CLI output together.
- Re-check privacy rules before adding any column that could contain user
  content, file paths, repository names, headers, or secrets.

## Common Changes

- Change usage parsing: edit `internal/proxy/capture.go`,
  `internal/proxy/sse.go`, or `internal/proxy/websocket.go`, with tests beside
  the files.
- Change pricing or model normalization: edit `internal/catalog/models.json`,
  `internal/catalog/catalog.go`, or `internal/cost/cost.go`.
- Change session behavior: look at `internal/store/sessions.go` and the
  `sessions` command and API handlers.
- Change model policy: edit `internal/policy/policy.go` (evaluation and cache),
  `internal/api/policy.go` (API handlers), and `internal/store/store.go`
  (persistence). Update `internal/policy/policy_test.go` for policy behavior and
  `internal/api/api_test.go` for API handlers.

Run `just test` for ordinary changes and `just all` before submitting broader
work.
