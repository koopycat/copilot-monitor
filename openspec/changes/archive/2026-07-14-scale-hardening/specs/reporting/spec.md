## ADDED Requirements

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
  with `WHERE session_id IN (...)`

## MODIFIED Requirements

### Requirement: Dashboard API

The system SHALL provide a read-only dashboard API with endpoints:
`/api/health`, `/api/stats`, `/api/cost`, `/api/today`, `/api/sessions`,
`/api/session/current`, `/api/stats/timeline`, `/api/export`, `/api/upstreams`,
`/api/config`, `/api/policy`, `/api/policy/models`, `/api/anomalies`.

#### Scenario: All API endpoints respond

- **WHEN** the dashboard API is started
- **THEN** all listed endpoints return JSON responses with
  `Access-Control-Allow-Origin: *`

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

#### Scenario: Rebuild sessions command

- **WHEN** user runs `copilot-monitor rebuild-sessions`
- **THEN** the sessions table is rebuilt from all requests ordered by timestamp
