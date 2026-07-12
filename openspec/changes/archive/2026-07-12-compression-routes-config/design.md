## Context

Headroom compression is currently configured via 5 CLI flags: `--headroom-url`,
`--headroom-timeout`, `--headroom-required`,
`--headroom-compress-user-messages`, `--headroom-target-ratio`. This scatters
compression config across the command line when it logically belongs with route
definitions.

The routes JSON is already the single source of truth for upstream behavior.
Compression is route-contextual — it only applies to `/chat/completions` and
`/v1/chat/completions`. Moving compression config into routes co-locates it with
the route it affects.

## Goals / Non-Goals

**Goals:**

- Eliminate all 5 `--headroom-*` CLI flags
- Move compression configuration into the routes JSON file as a `compression`
  block on each route
- Enable per-route compression (different providers/paths can have different
  settings)
- Maintain backward-compatible runtime behavior: fail-open by default, same
  status labels, same envelope handling
- Keep timeout hardcoded to 30s (loopback call with no practical need to tune)

**Non-Goals:**

- Change the Headroom wire protocol or API shape
- Add per-request compression configuration
- Support non-loopback compression endpoints
- Change the persistence schema or dashboard

## Decisions

### Compression config lives in routes JSON, not CLI flags

**Why**: Routes JSON is the single source of truth for upstream behavior.
Compression is a property of how a route's requests are handled. Co-locating
compression config with the route it affects eliminates the need to keep CLI
flags and route config in sync, and enables per-route granularity.

**Alternative considered**: Keep a single `--headroom` flag for the endpoint and
put tuning knobs in a separate config file. Rejected because it adds a second
config file and doesn't solve the per-route problem.

### Endpoint is `host:port`, not full URL

**Why**: Headroom enforces strict constraints (loopback-only, HTTP-only, path
must be `/v1/compress`). Requiring a full URL like
`http://127.0.0.1:8787/v1/compress` is redundant ceremony. The `host:port`
format (`127.0.0.1:8787`) captures the only variable part.

### Headroom clients are lazily constructed and cached per endpoint

**Why**: The handler shouldn't fail at startup if the headroom endpoint is
unreachable — compression is fail-open. Lazy construction means the first
eligible request triggers client creation. Caching avoids constructing a new
HTTP client on every request.

### Timeout is hardcoded to 30 seconds

**Why**: Headroom is a local loopback service that should respond in
milliseconds. A 30-second timeout is generous and there is no practical scenario
where a user needs to tune this. Removing the flag simplifies the interface.

## Risks / Trade-offs

- **Lazy construction means first request pays latency cost** → Negligible for a
  loopback HTTP client creation (<1ms)
- **Malformed endpoint causes repeated construction attempts** → Cache nil on
  failure; subsequent requests skip construction. User must fix config and
  restart
- **BREAKING: all `--headroom-*` flags removed** → Users must update their
  startup scripts. Migration is straightforward: move settings from CLI flags to
  a `compression` block in the routes JSON
