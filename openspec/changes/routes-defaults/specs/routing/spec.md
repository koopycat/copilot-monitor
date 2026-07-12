## MODIFIED Requirements

### Requirement: Configuration-driven routing

The system SHALL define routing entirely through a JSON route configuration
file. When no configuration file is provided, built-in default routes SHALL be
used. No provider routing SHALL be hardcoded beyond the built-in defaults.

#### Scenario: Routes are loaded from config

- **WHEN** the proxy starts with `--routes-config routes.json`
- **THEN** all routes are read from the JSON file and used for request matching

#### Scenario: Missing routes config loads defaults

- **WHEN** the proxy starts without `--routes-config`
- **THEN** built-in default routes (GitHub Copilot endpoints) are loaded and the
  startup banner indicates defaults are active
