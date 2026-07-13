<!-- markdownlint-disable MD041 -->

## Purpose

Provide an embedded browser dashboard for visualizing LLM usage with
period-based filtering, timeline charts, per-model breakdowns, session history,
policy management, and CSV export.

## Requirements

### Requirement: Overview metrics

The dashboard SHALL display an overview of captured usage including estimated
cost, total request count, and projected monthly cost for the selected period.

#### Scenario: Dashboard loads

- **WHEN** the dashboard page is opened
- **THEN** metric cards show estimated AI-credit cost, projected monthly cost,
  and total request count for the selected period

---

### Requirement: Period-based filtering

The dashboard SHALL support switching between predefined periods.

#### Scenario: Period changed

- **WHEN** the user clicks a period button (e.g., "7d", "30d")
- **THEN** all dashboard views refresh to show data only for the selected period

---

### Requirement: Usage timeline chart

The dashboard SHALL display a usage timeline chart with toggles for granularity
(day/hour) and metric (tokens/requests).

#### Scenario: Granularity toggle

- **WHEN** the user switches between "Day" and "Hour" granularity
- **THEN** the chart re-renders with the selected time bucket size

#### Scenario: Metric toggle

- **WHEN** the user switches between "Tokens" and "Requests" metrics
- **THEN** the chart Y-axis changes to the selected metric

---

### Requirement: Table section disclosure controls

The dashboard SHALL provide independent, keyboard-operable disclosure controls
for the Models and Recent Sessions table sections. Each control SHALL visibly
communicate whether its section is expanded or collapsed and SHALL retain a
visible keyboard focus indicator.

#### Scenario: Keyboard toggle

- **WHEN** a keyboard user focuses a table section disclosure control and
  activates it
- **THEN** only that section changes between expanded and collapsed while the
  other table section retains its state

---

### Requirement: Per-model breakdown

The dashboard SHALL display a per-model breakdown table showing model names,
request counts, token counts, cache hit ratios, latency, and estimated cost per
model. The Models section SHALL be expanded when the dashboard first loads and
independently collapsible thereafter.

#### Scenario: Models table

- **WHEN** captured data exists and the Models section is expanded
- **THEN** the table shows one row per model with requests, tokens, cache
  percentage, average latency, and cost

#### Scenario: Models section collapsed

- **WHEN** the user collapses the Models section
- **THEN** the model table is hidden while the Models section control remains
  visible

---

### Requirement: Recent sessions table

The dashboard SHALL display recent sessions in a table with start time,
duration, project label, request count, token count, and estimated cost. The
Recent Sessions section SHALL be expanded when the dashboard first loads and
independently collapsible thereafter.

#### Scenario: Sessions table

- **WHEN** sessions exist in the database and the Recent Sessions section is
  expanded
- **THEN** they are listed in reverse chronological order with summary fields

#### Scenario: Recent Sessions section collapsed

- **WHEN** the user collapses the Recent Sessions section
- **THEN** the sessions table is hidden while the Recent Sessions section
  control remains visible

---

### Requirement: Live session card

The dashboard SHALL display a live session card showing whether the most recent
session is active or idle, its duration, request count, token count, and
estimated cost.

#### Scenario: Active session

- **WHEN** the most recent session's last request was within the last 30 minutes
- **THEN** the card shows "● active" with the session details

#### Scenario: Idle session

- **WHEN** the most recent session is older than 30 minutes
- **THEN** the card shows "○ idle" with time since last activity

---

### Requirement: In-browser policy management

The dashboard SHALL support viewing and editing the model policy with mode
selection and model pattern input.

#### Scenario: Policy panel load

- **WHEN** the policy panel is expanded
- **THEN** the current mode and model patterns are displayed

#### Scenario: Policy change

- **WHEN** the user changes the mode or model patterns and saves
- **THEN** the new policy is persisted via `PUT /api/policy`

---

### Requirement: Route configuration display

The dashboard SHALL display the routes loaded from the configuration file with
path, upstream host, capture mode, and provider.

#### Scenario: Routes panel

- **WHEN** the routes panel is expanded
- **THEN** each configured route is shown with its path, upstream host, capture
  mode, and provider label

---

### Requirement: Upstream host filter

The dashboard SHALL provide an upstream host filter control that lists all
distinct upstream hosts from captured data and restricts all views to the
selected host.

#### Scenario: Upstream filter changed

- **WHEN** the user selects a different upstream host from the dropdown
- **THEN** all dashboard views refresh to show data only for the selected host

---

### Requirement: CSV export link

The dashboard SHALL provide a CSV export link that downloads captured request
metadata filtered by the currently selected period and upstream host.

#### Scenario: Export link clicked

- **WHEN** the user clicks the "Export CSV" link
- **THEN** a CSV file is downloaded with metadata for the current period and
  upstream filter

---

### Requirement: Auto-refresh

The dashboard SHALL auto-refresh its data at a regular interval.

#### Scenario: Periodic refresh

- **WHEN** the dashboard is open in a browser
- **THEN** it automatically refreshes data every 30 seconds

---

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
