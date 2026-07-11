<!-- markdownlint-disable MD041 -->

## Purpose

Route LLM API requests to upstream providers based on a JSON configuration file,
with model-based filtering, provider prefix stripping, and explicit capture
modes.

## Requirements

### Requirement: Configuration-driven routing

The system SHALL define routing entirely through a JSON route configuration
file. No provider routing SHALL be hardcoded.

#### Scenario: Routes are loaded from config

- **WHEN** the proxy starts with `--routes-config routes.json`
- **THEN** all routes are read from the JSON file and used for request matching

#### Scenario: Missing routes config

- **WHEN** the proxy starts without `--routes-config`
- **THEN** the process fails with a clear error message telling the user where
  to find sample configs

---

### Requirement: Unknown path rejection

Unknown inbound paths SHALL be rejected with an error response.

#### Scenario: Unknown path

- **WHEN** a request arrives at a path that matches no configured route
- **THEN** the proxy returns 502 Bad Gateway

---

### Requirement: Local paths

Local health and ping paths SHALL NOT be forwarded upstream. They SHALL return
responses directly from the proxy.

#### Scenario: Ping endpoint

- **WHEN** a request matches a route with `capture: "local"`
- **THEN** the proxy returns 200 OK without forwarding to any upstream

---

### Requirement: Explicit capture mode

Every configured route SHALL have an explicit capture mode: `usage`, `metadata`,
`none`, `tunnel`, or `local`.

#### Scenario: Validation of capture mode

- **WHEN** the route config is validated
- **THEN** any route with an unrecognized capture mode causes a validation error

---

### Requirement: Model-based filtering

Routes SHALL support optional model-based filtering with exact match and
`*`-prefix patterns. Routes without model filters SHALL match any model.

#### Scenario: Model filter matches

- **WHEN** a route specifies `models: ["gpt-4o"]` and a request body contains
  `"model": "gpt-4o"`
- **THEN** the route matches

#### Scenario: Prefix pattern matches

- **WHEN** a route specifies `models: ["gpt-*"]` and a request body contains
  `"model": "gpt-4o-mini"`
- **THEN** the route matches

#### Scenario: Model filter does not match

- **WHEN** a route specifies `models: ["gpt-4o"]` and a request body contains
  `"model": "claude-3-opus"`
- **THEN** the route does not match

#### Scenario: No model filter

- **WHEN** a route has no `models` field or an empty models array
- **THEN** the route matches any model

---

### Requirement: Provider label

Routes SHALL support an explicit `provider` label used in reports and cost
attribution.

#### Scenario: Provider label in reports

- **WHEN** a route has `"provider": "openai"`
- **THEN** the provider value is stored with captured requests and used in cost
  lookups and report output

---

### Requirement: Not-billed flag

Routes SHALL support a `not_billed` flag that marks request rows as zero-cost
regardless of token counts.

#### Scenario: Not-billed route

- **WHEN** a route has `"not_billed": true`
- **THEN** captured rows are stored with the not-billed flag and cost
  calculations return zero for those rows

---

### Requirement: Provider prefix stripping

Known provider prefixes in URL paths SHALL be stripped before route matching and
stored for provider attribution in reports. Recognized prefixes are `/copilot/`,
`/openai/`, and `/kilo/`.

#### Scenario: Copilot prefix stripped

- **WHEN** a request arrives at `/copilot/chat/completions`
- **THEN** the prefix `copilot` is extracted, the path `/chat/completions` is
  used for route matching, and the prefix is stored for reports

#### Scenario: No recognized prefix

- **WHEN** a request arrives at `/v1/chat/completions` with no recognized prefix
- **THEN** the full path is used for route matching and no prefix is stored

---

### Requirement: Provider prefix in model matching

The stripped provider prefix SHALL be passed to model-based route matching for
routes that filter by provider.

#### Scenario: Provider-specific route matching

- **WHEN** a route has `"provider": "openai"` and a request has prefix `openai`
- **THEN** the route is considered for matching

#### Scenario: Provider mismatch in route matching

- **WHEN** a route has `"provider": "openai"` and a request has prefix `copilot`
- **THEN** the route is excluded from matching

---

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
