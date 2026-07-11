## ADDED Requirements

### Requirement: Inspect CLI command

The system SHALL provide a `copilot-monitor inspect` CLI subcommand that reads
anomaly records from the database and displays them in a human-readable table or
as JSON.

#### Scenario: Inspect with default output

- **WHEN** user runs `copilot-monitor inspect`
- **THEN** anomaly records from the last 24 hours are displayed grouped by
  category and severity, with counts and the most recent detail per category

#### Scenario: Inspect with no anomalies

- **WHEN** user runs `copilot-monitor inspect` and no anomalies exist in the
  last 24 hours
- **THEN** the command prints "No anomalies found in the last 24 hours" and
  exits with code 0

---

### Requirement: Inspect time filtering

The `inspect` command SHALL support a `--since` flag accepting duration values
(e.g., `1h`, `24h`, `7d`) to filter anomalies by time window.

#### Scenario: Inspect with --since flag

- **WHEN** user runs `copilot-monitor inspect --since 1h`
- **THEN** only anomalies recorded within the last hour are displayed

---

### Requirement: Inspect category filtering

The `inspect` command SHALL support a `--category` flag to filter anomalies by
category (e.g., `unrouted_path`, `parse_error`, `auth_missing`).

#### Scenario: Inspect filtered by category

- **WHEN** user runs `copilot-monitor inspect --category unrouted_path`
- **THEN** only anomalies whose category equals `unrouted_path` are displayed

#### Scenario: Inspect with unknown category

- **WHEN** user runs `copilot-monitor inspect --category unknown_example`
- **THEN** the command prints an error indicating valid category values

---

### Requirement: Inspect severity filtering

The `inspect` command SHALL support a `--severity` flag to filter anomalies by
severity (e.g., `info`, `warn`, `error`).

#### Scenario: Inspect filtered by severity

- **WHEN** user runs `copilot-monitor inspect --severity error`
- **THEN** only anomalies with `severity = error` are displayed

---

### Requirement: Inspect JSON output

The `inspect` command SHALL support a `--json` flag that emits anomaly records
as a JSON array of objects with snake_case field names.

#### Scenario: Inspect with JSON output

- **WHEN** user runs `copilot-monitor inspect --json`
- **THEN** output is a valid JSON array of anomaly objects, each with fields
  including `id`, `ts`, `category`, `severity`, `path`, `model`, `detail`

---

### Requirement: Inspect alert-on-any mode

The `inspect` command SHALL support a `--alert-on-any` flag that causes the
command to exit with code 1 when any anomalies match the query criteria.

#### Scenario: Alert triggers

- **WHEN** user runs `copilot-monitor inspect --alert-on-any` and any anomaly
  exists in the last 24 hours
- **THEN** the command exits with code 1

#### Scenario: Alert does not trigger

- **WHEN** user runs `copilot-monitor inspect --alert-on-any` and no anomalies
  exist in the last 24 hours
- **THEN** the command exits with code 0

---

### Requirement: Inspect table output format

The `inspect` command's default human-readable output SHALL group anomalies by
category with counts and the most recent timestamp per category.

#### Scenario: Human-readable output structure

- **WHEN** user runs `copilot-monitor inspect --since 24h` and two categories of
  anomalies exist
- **THEN** a header shows the time window, followed by groups sorted by severity
  (error > warn > info), each showing the category, count, and most recent
  occurrence
