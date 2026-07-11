## Context

After removing the built-in Copilot fallback in favor of provider prefix
routing, every endpoint path must be explicitly listed in the routes config. For
providers like GitHub Copilot that expose many paths (`/models`,
`/models/session`, `/agents`, `/responses`, `/embeddings`, `/chat/completions`,
etc.), this forces verbose config files. The old `copilotRoutePath()` hardcoded
fallback handled this but was provider-specific.

The `Router` currently uses two data structures: `exactRoutes map[string]Route`
for `Match()` and `entries []routeEntry` (exact, then prefix, longest-first) for
`MatchModel()`.

## Goals / Non-Goals

**Goals:**

- Let users define a per-provider fallback route with no path
- Specific path routes always take precedence over the provider default
- Provider defaults support the same fields as regular routes (capture,
  upstream_path_prefix, not_billed, models)
- Config validation catches misconfigured defaults

**Non-Goals:**

- Global catch-all (no provider): always rejected -- this would silently mask
  unknown provider prefixes
- Changing the behavior of existing path routes
- Supporting multiple defaults per provider

## Decisions

### Decision 1: Default route is identified by absence of `path`

A route entry without `path` and with `provider` + `upstream_host` is a provider
default. This is the simplest signal -- no new field needed.

**Alternatives considered:**

- A `default: true` boolean field: adds noise, easy to forget, conflicts with
  `path`
- A separate `defaults` top-level key: adds schema complexity for a single list

### Decision 2: Store defaults in a separate `map[string]Route` in Router

`Router.defaultRoutes map[string]Route` keyed by provider. Checked in
`MatchModel` only after the path-based `entries` loop finds nothing. Keeps the
hot path unchanged and the fallback explicit.

**Alternatives considered:**

- Append defaults to `entries` as prefix matches: `"/"` prefix would catch
  everything but would need special ordering to avoid matching before specific
  paths
- Single `defaultRoute` field: doesn't support per-provider defaults

### Decision 3: Default route resolution happens in `MatchModel`, not `ServeHTTP`

`MatchModel` already receives the provider and path. Adding the fallback there
keeps the callers unchanged. The `Match()` method (used by `combinedDashProxy`)
does not support provider matching, so defaults won't affect dashboard routing.

### Decision 4: Validation happens during config loading (`proxy.LoadConfig`)

A default route without `provider` or without `upstream_host` fails at startup.
This matches the existing pattern of catching config errors early.

## Risks / Trade-offs

- [Risk] User accidentally creates a default route that catches more than
  intended → Mitigation: provider default only matches after specific routes
  fail
- [Risk] Two default entries for the same provider → Mitigation: first one wins
  (insertion order), warning during validation
- [Risk] default route with `capture: "none"` means no visibility into unmatched
  paths → Mitigation: log a warning when a default route matches (already logged
  as part of normal request flow)
