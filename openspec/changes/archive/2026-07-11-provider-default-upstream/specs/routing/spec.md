## ADDED Requirements

### Requirement: Provider default upstream

A route entry with only `provider` and `upstream_host` (no `path`) SHALL act as
a default fallback route for that provider. When no specific path route matches,
the provider default route SHALL be used, forwarding the request to the
configured upstream host with the original path.

#### Scenario: Provider default catches unmatched path

- **WHEN** a route config has
  `{ "provider": "copilot", "upstream_host": "api.githubcopilot.com" }` and a
  request arrives at `/copilot/models/session` with no specific route for
  `/models/session` with provider `copilot`
- **THEN** the request is forwarded to `api.githubcopilot.com` using the path
  `/models/session`

#### Scenario: Specific path takes precedence over default

- **WHEN** a route config has both
  `{ "provider": "copilot", "path": "/models", "upstream_host": "api.githubcopilot.com" }`
  and a provider default
  `{ "provider": "copilot", "upstream_host": "fallback.example.com" }`, and a
  request arrives at `/copilot/models`
- **THEN** the specific route is used (`api.githubcopilot.com`), not the default

#### Scenario: Provider default with different provider does not match

- **WHEN** a route config has
  `{ "provider": "copilot", "upstream_host": "api.githubcopilot.com" }` and a
  request arrives at `/openai/v1/chat/completions`
- **THEN** the route does NOT match, and the request gets 502 (unless another
  route matches)

#### Scenario: Multiple provider defaults are independent

- **WHEN** a route config has defaults for `copilot` and `openai`
- **THEN** requests with prefix `copilot` fall through to the copilot default
  and requests with prefix `openai` fall through to the openai default

#### Scenario: Provider default validation

- **WHEN** a route entry has no `path` field
- **THEN** it SHALL have both `provider` and `upstream_host` set, otherwise
  config validation fails

#### Scenario: Provider default without upstream errors on validate

- **WHEN** a route entry has `provider` but no `path` and no `upstream_host`
- **THEN** config validation SHALL fail with a message indicating that a
  provider default route requires `upstream_host`

#### Scenario: Provider default with capture field

- **WHEN** a provider default route specifies a `capture` mode
- **THEN** requests matched by the default inherit that capture mode (defaulting
  to `usage` if unspecified)
