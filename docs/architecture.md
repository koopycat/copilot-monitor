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
2. On startup, `run.go` loads routes in priority order: explicit
   `--routes-config` flag → `~/.config/copilot-monitor/routes.json` (XDG config
   dir) → built-in defaults from `internal/proxy.DefaultRoutes()`. The startup
   banner indicates which source is active.
3. `internal/proxy.Handler.ServeHTTP` assigns a request ID, records the start
   time, strips a known provider prefix (`/copilot`, `/openai`, `/kilo`) from
   the URL path via `StripProviderPrefix`, and routes the stripped path through
   `internal/proxy.Router.MatchModel` along with the detected provider name.
4. `Router.MatchModel` matches configured routes against the stripped path, with
   optional `provider` and `models` filters. Provider-default routes catch
   unmatched paths for a given provider prefix.
5. `internal/proxy/forward.go` reads the request body only long enough to
   extract safe metadata such as `model` and `stream`.
6. `internal/policy.Policy.Allow` evaluates the global model policy. When a
   request model matches a configured blocklist, the proxy returns HTTP 403 with
   a JSON error body, persists the blocked attempt, and never contacts the
   upstream.
7. If the matched route has a `compression` block with an `endpoint`, eligible
   OpenAI-compatible chat requests are compressed via the loopback Headroom
   `/v1/compress` endpoint after routing and policy checks. Only the model and
   supported messages are sent; provider headers and the rest of the request
   envelope stay in the proxy. A successful response replaces `messages` while
   preserving the other request fields. Compression is fail-open by default;
   `"required": true` returns HTTP 502 instead.
8. The proxy builds an HTTPS upstream request from the final body. Copilot
   clients may use either prefixed paths (e.g. `/copilot/chat/completions`) or
   bare paths (e.g. `/chat/completions`); the provider prefix is stripped before
   internal routing. For built-in Copilot routes the upstream is
   `api.githubcopilot.com` or `copilot-proxy.githubusercontent.com`. For
   config-driven routes the upstream is whatever host the config specifies, with
   an optional `upstream_path_prefix` prepended to the outgoing path. Strip
   hop-by-hop headers and disable compression so response usage can be observed.
9. Responses are streamed back to the client as they arrive. For JSON and SSE
   responses, `internal/proxy/sse.go` watches chunks for `usage` and response
   `model` fields without retaining the full response.
10. `internal/proxy/server.go` persists only metadata and token counts through
    `internal/store.InsertRequest` when the route is configured to capture.
    WebSocket traffic (Copilot `/responses`) is inspected frame-by-frame by
    `internal/proxy/websocket.go` for `response.create` (model) and
    `response.completed` (usage) events, and persisted the same way as HTTP
    completions.
11. CLI reporting commands and the dashboard API read aggregate data from
    `internal/store`.

Capture behavior is defined per route in `Router.MatchModel` (config-driven or
built-in defaults, after provider-prefix stripping):

- `CaptureUsage`: persist only when usage tokens are found, for chat, agents,
  messages, completions, and WebSocket responses.
- `CaptureMetadata`: persist request metadata without requiring usage, currently
  for embeddings.
- `CaptureNone`, `CaptureTunnel`, `CaptureLocal`: do not persist request rows
  (WebSocket tunneling when needed, model metadata, local ping).
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

- **WebSocket gap**: WebSocket `/responses` traffic bypasses model policy
  because the model is only known after the connection is established (see
  [Known limitations](../.github/SECURITY.md#known-limitations)).

Policy management is through the dashboard API: `GET/PUT /api/policy` and model
discovery via `GET /api/policy/models`.

## Persistence and Privacy Rules

The core invariant is: do not store prompts, completions, source code, auth
material, cookies, or API keys. The `requests` table stores timestamps, endpoint
and path, upstream host, model, stream flag, HTTP status, latency, token counts,
project label, and an optional session link. The `sessions` table is maintained
at request insertion time using a 30-minute inactivity gap and can be rebuilt
with the offline `rebuild-sessions` command. Request bodies are parsed for
metadata and then forwarded, but are never persisted. Response bodies are
streamed to the client while observers look only for usage fields. Debug usage
logs are opt-in via `--usage-debug-log` and must stay metadata-only;
`SafeHeaders` redacts sensitive response headers. Keep listeners loopback-only
by default (`127.0.0.1`) unless there is a clear security reason to expose them.

Headroom is a separate local process and may retain original content according
to its own configuration. Copilot Monitor persists estimated compression token
metrics (`compression_status`, `compression_original_tokens`,
`compression_final_tokens`, `compression_latency_ms`) as nullable columns on the
`requests` table. Provider response usage remains authoritative for cost
reporting; compression savings are labeled as estimates and are not mixed into
billed usage.

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
- Add or update `internal/store/*_test.go` to cover insert and query behavior.
- Re-check privacy rules before adding any column that could contain user
  content, file paths, repository names, headers, or secrets.

## Common Changes

- Add or change a proxied route: for built-in Copilot routes, edit
  `internal/proxy/defaults.go` (`DefaultRoutes()`). For configurable third-party
  routes, create or edit a JSON routes file. When `--routes-config` is omitted,
  the proxy loads `~/.config/copilot-monitor/routes.json` (created by
  `copilot-monitor init`) before falling back to built-in defaults. To add a new
  known provider prefix, edit `KnownProviders` and `StripProviderPrefix` in
  `router.go`.
- Route definitions: `internal/proxy/config.go` (types and loader),
  `internal/proxy/router.go` (matching logic). Update router tests when changing
  matching behavior.
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
