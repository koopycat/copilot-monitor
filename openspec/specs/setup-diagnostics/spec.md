<!-- markdownlint-disable MD041 -->

## Purpose

Provide a non-mutating CLI diagnostic tool (`doctor`) that inspects the local
monitor setup without modifying the database, filesystem, or running services.

## Requirements

### Requirement: Standalone local setup diagnosis

The system SHALL provide a `doctor` CLI subcommand that checks the local monitor
setup without creating, migrating, or modifying the configured SQLite database.
By default it SHALL check the local proxy, dashboard, and database path. It
SHALL support explicit opt-out flags for a service that the user did not start.

#### Scenario: Healthy combined local setup

- **WHEN** the proxy and dashboard are running on their default addresses and
  the user runs `copilot-monitor doctor`
- **THEN** the command reports passing proxy and dashboard checks
- **AND** exits 0

#### Scenario: Proxy is unavailable

- **WHEN** the proxy health endpoint cannot be reached
- **THEN** the command reports a failed proxy check with a command to start the
  proxy
- **AND** exits 1

#### Scenario: Dashboard intentionally omitted

- **WHEN** the user runs `copilot-monitor doctor --skip-dashboard`
- **THEN** the dashboard check is reported as skipped
- **AND** it does not make the command fail

### Requirement: Non-mutating database inspection

The diagnostic command SHALL inspect the configured database path without
calling the store initialization path or changing filesystem/database state. A
missing database before the first capture SHALL be reported as a warning, not a
failure.

#### Scenario: Database has not been created yet

- **WHEN** the configured database path does not exist
- **THEN** the command reports that it will be created on the first capture
- **AND** it does not create the file or parent directories

#### Scenario: Existing unreadable or invalid database file

- **WHEN** the configured database path is inaccessible or is not a SQLite
  database file
- **THEN** the command reports a failed database check with remediation
- **AND** exits 1

### Requirement: Optional upstream reachability check

The diagnostic command SHALL only test an external upstream when the user
provides `--upstream`. The check SHALL use a bounded TCP connection and SHALL
not send an authenticated HTTP request or request body.

#### Scenario: Upstream check is omitted by default

- **WHEN** the user runs `copilot-monitor doctor` without `--upstream`
- **THEN** the report marks the upstream check as skipped
- **AND** no external connection is attempted

#### Scenario: Unreachable selected upstream

- **WHEN** the user supplies an unreachable `--upstream` host
- **THEN** the command reports a failed upstream check and exits 1

### Requirement: Machine-readable diagnostic output

The `doctor` command SHALL support `--json` output with an overall `ok` field
and a stable list of named checks using snake_case JSON keys. It SHALL use exit
code 2 for invalid command input.

#### Scenario: JSON diagnostics

- **WHEN** the user runs `copilot-monitor doctor --json`
- **THEN** stdout contains valid JSON with `ok` and `checks` fields

#### Scenario: Invalid timeout

- **WHEN** the user passes a non-positive `--timeout`
- **THEN** the command exits 2 and reports actionable flag guidance
