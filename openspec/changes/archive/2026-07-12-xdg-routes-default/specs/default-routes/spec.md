## MODIFIED Requirements

### Requirement: Built-in default routes

The system SHALL provide built-in default routes that activate when no
`--routes-config` is specified and no default config file exists. When no
`--routes-config` is provided, the system SHALL first attempt to load
`$XDG_CONFIG_HOME/copilot-monitor/routes.json` (falling back to
`~/.config/copilot-monitor/routes.json`). If the file exists and is valid, its
routes SHALL be used. If the file does not exist or is invalid, built-in default
routes SHALL be used instead.

#### Scenario: Default routes activate when no config provided and no default file

- **WHEN** `copilot-monitor run` is started without `--routes-config` and
  `~/.config/copilot-monitor/routes.json` does not exist
- **THEN** the proxy uses the built-in default routes and the startup banner
  indicates "using built-in default routes"

#### Scenario: Default config file is loaded automatically

- **WHEN** `copilot-monitor run` is started without `--routes-config` and
  `~/.config/copilot-monitor/routes.json` exists and is valid
- **THEN** routes from the config file are used and the startup banner indicates
  "using routes from config file"

#### Scenario: Invalid default config file falls back to built-in

- **WHEN** `copilot-monitor run` is started without `--routes-config` and
  `~/.config/copilot-monitor/routes.json` exists but is invalid
- **THEN** a warning is logged and built-in default routes are used

#### Scenario: Explicit --routes-config overrides default file

- **WHEN** `copilot-monitor run` is started with `--routes-config custom.json`
- **THEN** the default config file is NOT checked; only `custom.json` is loaded

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
