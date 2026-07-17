<!-- markdownlint-disable MD041 -->

## Purpose

Surface captured LLM usage data through CLI subcommands and a read-only
dashboard API, with time/project/upstream filtering, cost estimation, CSV
export, and configuration tooling.

## Requirements

### Requirement: CLI commands

The system SHALL provide read-only CLI subcommands: `stats`, `cost`, `today`,
`sessions`, `live`, `export`. The system SHALL also provide a `rebuild-sessions`
subcommand for offline session reconstruction. Commands SHALL NOT rebuild
sessions during read operations; session data SHALL be read from the
incrementally-maintained `sessions` table.

#### Scenario: Stats command

- **WHEN** user runs `copilot-monitor stats --since 7d`
- **THEN** a table of model/endpoint usage statistics is printed covering the
  last 7 days

#### Scenario: Cost command

- **WHEN** user runs `copilot-monitor cost`
- **THEN** a published token-rate estimate per model is printed, with fallback
  pricing and not-billed rows clearly marked
- **AND** the output states that it is not invoice reconciliation

#### Scenario: Today command

- **WHEN** user runs `copilot-monitor today`
- **THEN** usage statistics since midnight local time are printed

#### Scenario: Sessions command

- **WHEN** user runs `copilot-monitor sessions --limit 10`
- **THEN** the 10 most recent sessions are printed with start time, duration,
  project, request count, and token count

#### Scenario: Live command

- **WHEN** user runs `copilot-monitor live`
- **THEN** the current active or idle session is printed with per-model
  breakdown

#### Scenario: Live watch mode

- **WHEN** user runs `copilot-monitor live --watch`
- **THEN** the display auto-refreshes every 2 seconds until Ctrl+C

#### Scenario: Export command

- **WHEN** user runs `copilot-monitor export`
- **THEN** CSV output with metadata columns is written to stdout

#### Scenario: Rebuild sessions command

- **WHEN** user runs `copilot-monitor rebuild-sessions`
- **THEN** the sessions table is rebuilt from all requests ordered by timestamp

---

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

---

### Requirement: JSON output

The following commands SHALL support `--json` for machine-readable output:
`stats`, `cost`, `today`, `sessions`, `live`.

#### Scenario: JSON output for empty results

- **WHEN** `--json` is specified and no data exists
- **THEN** a valid JSON empty array `[]` is emitted

---

### Requirement: Usage missing footnote

Reporting commands SHALL display a footnote when requests with `usage_missing`
exist in the result set.

#### Scenario: Usage-missing footnote

- **WHEN** the stats or cost command is run and the database contains requests
  with `usage_missing=1`
- **THEN** a footnote like `(N request(s) had no usage data)` is printed

---

### Requirement: Exit codes

Exit codes SHALL follow convention: 0 for success, 1 for runtime error, 2 for
usage error (bad flags/args), 130 for SIGINT.

#### Scenario: Bad flag

- **WHEN** an unknown or invalid flag is passed to a command
- **THEN** exit code is 2

#### Scenario: Runtime failure

- **WHEN** a command fails due to a database or network error
- **THEN** exit code is 1

---

### Requirement: Dashboard API

The system SHALL provide a read-only dashboard API with endpoints:
`/api/health`, `/api/stats`, `/api/cost`, `/api/today`, `/api/sessions`,
`/api/session/current`, `/api/stats/timeline`, `/api/export`, `/api/upstreams`,
`/api/policy`, `/api/policy/models`, `/api/anomalies`.

#### Scenario: All API endpoints respond

- **WHEN** the dashboard API is started
- **THEN** all listed endpoints return JSON responses with
  `Access-Control-Allow-Origin: *`

---

### Requirement: Anomaly API endpoint

The system SHALL provide a `/api/anomalies` endpoint returning anomalies ordered
by timestamp descending, supporting optional category and severity filters.

#### Scenario: Anomalies endpoint

- **WHEN** a GET request is made to `/api/anomalies`
- **THEN** a JSON array of the most recent 50 anomalies is returned

#### Scenario: Anomalies with category filter

- **WHEN** a GET request is made to `/api/anomalies?category=unrouted_path`
- **THEN** only anomalies matching that category are returned

#### Scenario: Anomalies with severity filter

- **WHEN** a GET request is made to `/api/anomalies?severity=error`
- **THEN** only anomalies with severity "error" are returned

---

### Requirement: Sanitized error responses

API error responses SHALL return a generic error message to the client. The full
error details SHALL be logged server-side only.

#### Scenario: Database error

- **WHEN** an API handler encounters a database error
- **THEN** the HTTP response has status 500 with body
  `{"error":"internal server error"}` and the full error is logged to stderr

---

### Requirement: Batched session model queries

The sessions API SHALL retrieve per-session model stats using a single batched
query rather than one query per session.

#### Scenario: Sessions list with model stats

- **WHEN** `/api/sessions` returns 20 sessions
- **THEN** model stats for all 20 sessions are retrieved in a single SQL query
  with `WHERE session_id IN (...)`.

---

### Requirement: Time range filtering

Reports SHALL support filtering by time range with both lower bound (`since`)
and upper bound (`until`) parameters.

#### Scenario: Since parameter as duration

- **WHEN** `?since=7d` is passed to a reporting endpoint
- **THEN** results are filtered to requests from the last 7 days

#### Scenario: Since and until parameters

- **WHEN** both `?since=...&until=...` are passed
- **THEN** results are filtered to the specified time window

#### Scenario: No time filter

- **WHEN** no time parameters are passed
- **THEN** all historical data is included

---

### Requirement: Project and endpoint filtering

Reports SHALL support filtering by project and endpoint where applicable.

#### Scenario: Project filter

- **WHEN** `?project=myproject` is passed to a reporting endpoint
- **THEN** results are limited to requests with that project label

#### Scenario: Endpoint filter

- **WHEN** `?endpoint=chat` is passed
- **THEN** results are limited to requests from that endpoint

---

### Requirement: Upstream host filtering

Reports SHALL support filtering by upstream host where applicable. A dedicated
`/api/upstreams` endpoint SHALL return all distinct upstream hosts.

#### Scenario: Upstream filter

- **WHEN** `?upstream=api.githubcopilot.com` is passed to stats, cost, timeline,
  or export
- **THEN** results are limited to requests to that upstream

#### Scenario: Upstream discovery

- **WHEN** a GET request is made to `/api/upstreams`
- **THEN** a JSON array of all distinct upstream hostnames is returned

---

### Requirement: Startup banner

The `run` command SHALL emit startup information showing the address, upstream
host, database path, and a copy-pasteable verification curl command.

#### Scenario: Proxy startup

- **WHEN** `copilot-monitor run --upstream api.githubcopilot.com` starts
- **THEN** stderr gets:
  `copilot-monitor: listening on 127.0.0.1:7733 (upstream api.githubcopilot.com) - curl http://127.0.0.1:7733/_ping`

#### Scenario: Combined mode startup

- **WHEN** `--dashboard` is passed to `run`
- **THEN** both the dashboard UI URL and API URL are printed as separate lines

#### Scenario: Database path displayed

- **WHEN** the proxy starts
- **THEN** the resolved absolute database path is printed

---

### Requirement: Standalone serve mode

A `serve` subcommand SHALL start the read-only dashboard API and UI on a
configurable port without the proxy.

#### Scenario: Serve started

- **WHEN** `copilot-monitor serve` is run
- **THEN** the dashboard API listens on 127.0.0.1:7734 and the embedded UI is
  served at `/`

---

### Requirement: Combined proxy and dashboard

The `run` command's `--dashboard` flag SHALL serve both the proxy and dashboard
on the same port.

#### Scenario: Combined mode

- **WHEN** `copilot-monitor run --dashboard --upstream api.githubcopilot.com`
- **THEN** paths `/_ping` and `/_health` are handled locally, all other paths
  are forwarded to the upstream, and the dashboard and API are served

---

### Requirement: Provider-specific cost fallback

Cost output SHALL be labeled as a published token-rate estimate and include
machine-readable metadata describing its currency, rate source when available,
calculation basis, and non-invoice billing scope. The estimate SHALL use a
two-tier fallback: when a model is not in the catalog, provider-specific
fallback rates are tried first, then a generic fallback.

#### Scenario: Exact model pricing

- **WHEN** the model is in the pricing catalog
- **THEN** the model's exact pricing rates are used
- **AND** the cost total identifies the result as a published token-rate
  estimate rather than invoice reconciliation

#### Scenario: Provider fallback

- **WHEN** the model is not in the catalog but a provider-specific fallback rate
  exists
- **THEN** the provider fallback rate is used and the row is marked as fallback

#### Scenario: Generic fallback

- **WHEN** the model is not in the catalog and no provider fallback exists
- **THEN** the global fallback rate is used and the row is marked as fallback

#### Scenario: Machine-readable estimate semantics

- **WHEN** the cost command or `/api/cost` returns JSON
- **THEN** the total includes `estimate.currency`, `estimate.basis`, and
  `estimate.billing_scope`
- **AND** `estimate.rate_source` is included when the catalog declares one

---

### Requirement: CSV export

CSV export SHALL include stored metadata fields for rows meeting export filters
and omit bodies and secrets.

#### Scenario: CLI export

- **WHEN** `copilot-monitor export --since 30d` is run
- **THEN** a CSV with header row and metadata columns is written to stdout

#### Scenario: API export

- **WHEN** a GET request is made to `/api/export`
- **THEN** a CSV with Content-Disposition header is returned
