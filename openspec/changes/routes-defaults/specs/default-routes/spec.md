## ADDED Requirements

### Requirement: Built-in default routes

The system SHALL provide built-in default routes that activate when
`--routes-config` is not specified. The default routes SHALL cover GitHub
Copilot API endpoints with usage capture enabled.

#### Scenario: Default routes activate when no config provided

- **WHEN** `copilot-monitor run` is started without `--routes-config`
- **THEN** the proxy uses the built-in default routes and the startup banner
  indicates "using built-in default routes"

#### Scenario: Default routes are overridden by config file

- **WHEN** `copilot-monitor run` is started with `--routes-config routes.json`
- **THEN** the built-in default routes are NOT loaded; only routes from the
  config file are used

#### Scenario: Default routes cover all Copilot endpoints

- **WHEN** built-in default routes are active
- **THEN** the following paths are routable: `/chat/completions`, `/agents/*`,
  `/models`, `/models/session`, `/responses`, `/embeddings`, `/v1/engines/*`,
  `/v1/completions`, `/v1/messages/*`, and `/_ping`

#### Scenario: Default routes use correct upstream hosts

- **WHEN** built-in default routes are active
- **THEN** chat, agents, models, embeddings, and Anthropic-messages paths route
  to `api.githubcopilot.com`, and legacy completions paths route to
  `copilot-proxy.githubusercontent.com`

---

### Requirement: Default routes inspection

The system SHALL provide a `--routes-config-defaults` flag that prints the
built-in default routes as JSON to stdout and exits.

#### Scenario: Print defaults as JSON

- **WHEN** `copilot-monitor run --routes-config-defaults` is executed
- **THEN** the built-in default routes are printed as a valid JSON routes
  configuration to stdout and the process exits with code 0

#### Scenario: Defaults output is valid routes config

- **WHEN** the output of `--routes-config-defaults` is saved to a file and used
  as `--routes-config`
- **THEN** the proxy behaves identically to running without `--routes-config`
