## ADDED Requirements

### Requirement: Reproducible demo GIF generation

The repo SHALL provide a `just demo-gifs` target that regenerates all README
demo GIFs from synthetic seed data. Each demo SHALL have a corresponding `.tape`
file in `demo/` that drives vhs.

#### Scenario: Regenerating all demo GIFs

- **WHEN** a developer runs `just demo-gifs`
- **THEN** both `demo/copilot-monitor.gif` and `demo/copilot-monitor-nolive.gif`
  are regenerated from their respective `.tape` files

#### Scenario: Seed data is self-contained

- **WHEN** `just demo-gifs` runs
- **THEN** it creates a fresh `/tmp/demo.db` with synthetic data via
  `go run ./demo/seed/` before generating GIFs
- **AND** no real user data or credentials are embedded in the GIFs

### Requirement: CLI overview demo GIF

The repo SHALL include a `demo/copilot-monitor.gif` that walks through the main
CLI reporting commands (`version`, `today`, `stats`, `cost`, `live`) against
synthetic data.

#### Scenario: Overview GIF shows reporting commands

- **WHEN** the overview GIF is played
- **THEN** it displays output from `copilot-monitor version`, `today`, `stats`,
  `cost`, and `live` in sequence
- **AND** all output comes from the synthetic seed database

### Requirement: Verbose proxy output demo GIF

The repo SHALL include a `demo/copilot-monitor-nolive.gif` that shows the proxy
running with `--no-live`, where per-request log lines appear in the terminal as
commands are typed against the proxy.

#### Scenario: No-live GIF shows per-request logging

- **WHEN** the no-live GIF is played
- **THEN** it shows the proxy starting with `--no-live`
- **AND** per-request log lines (method, path, status, latency, model, tokens)
  appear as typed commands hit the proxy

### Requirement: GIFs embedded in README

The README SHALL embed both demo GIFs with brief captions explaining what each
shows.

#### Scenario: README shows both demos

- **WHEN** a visitor views the README on GitHub
- **THEN** both `demo/copilot-monitor.gif` and `demo/copilot-monitor-nolive.gif`
  are displayed inline
- **AND** each GIF has a caption describing its content
