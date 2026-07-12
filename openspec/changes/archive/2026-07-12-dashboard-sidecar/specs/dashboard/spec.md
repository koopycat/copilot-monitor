## ADDED Requirements

### Requirement: Dashboard sidecar on separate port

When `copilot-monitor run --dashboard` is used, the system SHALL start a
separate HTTP listener on port 7734 serving the dashboard API and UI. The
dashboard SHALL share the proxy's SQLite store. Both listeners SHALL shut down
gracefully on SIGINT or SIGTERM.

#### Scenario: Dashboard sidecar starts with run

- **WHEN** `copilot-monitor run --dashboard` is executed
- **THEN** the proxy listens on 7733 and the dashboard listens on 7734

#### Scenario: Dashboard sidecar shares store

- **WHEN** proxy-captured data is written to SQLite
- **THEN** the dashboard API immediately reflects the new data

#### Scenario: Dashboard sidecar shuts down with proxy

- **WHEN** SIGINT is sent to the `run` process
- **THEN** both the proxy and dashboard listeners stop gracefully
