<!-- markdownlint-disable MD041 -->

## Purpose

Surface captured LLM usage data through CLI subcommands and a read-only
dashboard API, with time/project/upstream filtering, cost estimation, CSV
export, and configuration tooling.

## Requirements

### Requirement: CLI commands

The system SHALL provide read-only CLI subcommands: `stats`, `cost`, `today`,
`sessions`, `live`, `export`. Commands MAY rebuild derived session summaries
before reporting but SHALL NOT alter captured request rows.

#### Scenario: Stats command

- **WHEN** user runs `copilot-monitor stats --since 7d`
- **THEN** a table of model/endpoint usage statistics is printed covering the
  last 7 days

#### Scenario: Cost command

- **WHEN** user runs `copilot-monitor cost`
- **THEN** estimated list-price cost per model is printed, with fallback pricing
  and not-billed rows clearly marked

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
`/api/config`, `/api/policy`, `/api/policy/models`.

#### Scenario: All API endpoints respond

- **WHEN** the dashboard API is started
- **THEN** all listed endpoints return JSON responses with
  `Access-Control-Allow-Origin: *`

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

- **WHEN** `?upstream=api.openai.com` is passed to stats, cost, timeline, or
  export
- **THEN** results are limited to requests to that upstream

#### Scenario: Upstream discovery

- **WHEN** a GET request is made to `/api/upstreams`
- **THEN** a JSON array of all distinct upstream hostnames is returned

---

### Requirement: Config validation command

A `validate` subcommand SHALL check a route config file for errors without
starting the proxy.

#### Scenario: Valid config

- **WHEN** `copilot-monitor validate --routes-config valid.json`
- **THEN** a message confirming the config is valid is printed, no server is
  started, exit code 0

#### Scenario: Invalid config

- **WHEN** `copilot-monitor validate --routes-config invalid.json`
- **THEN** the specific validation error is printed, no server is started, exit
  code 1

---

### Requirement: Init command

An `init` subcommand SHALL create a starter route config file with auto-detected
providers from the environment.

#### Scenario: Providers detected

- **WHEN** API key environment variables are present
- **THEN** routes are created only for detected providers

#### Scenario: No providers detected

- **WHEN** no API keys are found in the environment
- **THEN** a generic stub with model filters is created as a template

#### Scenario: Existing config

- **WHEN** the config file already exists and `--force` is not set
- **THEN** the command refuses to overwrite and exits with an error

---

### Requirement: Startup banner

The `run` command SHALL emit startup information showing the address, route
count, database path, and a copy-pasteable verification curl command.

#### Scenario: Proxy startup

- **WHEN** `copilot-monitor run --routes-config ...` starts
- **THEN** stderr gets:
  `copilot-monitor: listening on 127.0.0.1:7733 (N routes) - curl http://127.0.0.1:7733/_ping`

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

- **WHEN** `copilot-monitor run --dashboard --routes-config ...`
- **THEN** paths matching proxy routes are forwarded to upstreams, all other
  paths serve the dashboard and API

---

### Requirement: Provider-specific cost fallback

Cost output SHALL be labeled as an estimate using a two-tier fallback: when a
model is not in the catalog, provider-specific fallback rates are tried first,
then a generic fallback.

#### Scenario: Exact model pricing

- **WHEN** the model is in the pricing catalog
- **THEN** the model's exact pricing rates are used

#### Scenario: Provider fallback

- **WHEN** the model is not in the catalog but a provider-specific fallback rate
  exists
- **THEN** the provider fallback rate is used and the row is marked as fallback

#### Scenario: Generic fallback

- **WHEN** the model is not in the catalog and no provider fallback exists
- **THEN** the global fallback rate is used and the row is marked as fallback

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
