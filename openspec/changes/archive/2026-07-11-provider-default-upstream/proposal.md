## Why

After provider prefix routing was introduced, every route path must be
explicitly listed with a matching provider. Unknown paths return 502. This
forces users to enumerate every endpoint (`/models`, `/models/session`,
`/agents`, `/responses`, `/embeddings`, etc.) for each provider, even when all
traffic for a provider goes to a single upstream host. The old built-in Copilot
fallback handled this but was provider-specific. We need a provider-generic
default-upstream mechanism.

## What Changes

- New route form:
  `{ "provider": "copilot", "upstream_host": "api.githubcopilot.com" }` -- a
  default/fallback route for a provider with no `path` field
- Specific path routes take precedence over the provider default
- The existing `provider` field on path routes continues to work unchanged
- Default routes are validated: must have `provider` set, must not have `path`,
  must have `upstream_host`
- **BREAKING**: None. This is purely additive.

## Capabilities

### New Capabilities

<!-- No new capability -- this is a routing refinement, not a new domain -->

### Modified Capabilities

- `routing`: Default upstream per provider -- a route with only `provider` and
  `upstream_host` (no `path`) acts as a fallback for that provider's unmatched
  paths.

## Impact

- `internal/proxy/router.go`: NewRouter, MatchModel, route config parsing
- `openspec/specs/routing/spec.md`: New requirements and scenarios
- `examples/routes/github-copilot.json`: Simplify by using provider default
- Config validation (`proxy validate` / startup checks)
