<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Anomaly API endpoint

The system SHALL provide a `/api/anomalies` endpoint returning anomalies ordered
by timestamp descending.

#### Scenario: Anomalies endpoint with no filter

- **WHEN** a GET request is made to `/api/anomalies`
- **THEN** a JSON array of the most recent 50 anomalies is returned, sorted by
  `ts` descending

#### Scenario: Anomalies endpoint with category filter

- **WHEN** a GET request is made to `/api/anomalies?category=unrouted_path`
- **THEN** only anomalies matching that category are returned

#### Scenario: Anomalies endpoint with severity filter

- **WHEN** a GET request is made to `/api/anomalies?severity=error`
- **THEN** only anomalies with severity "error" are returned

#### Scenario: Empty anomalies

- **WHEN** no anomalies exist in the database
- **THEN** the endpoint returns an empty JSON array `[]`

---

### Requirement: Dashboard anomaly feed

The dashboard SHALL display a feed of the 10 most recent anomalies, each showing
category, severity, timestamp, and a brief detail message.

#### Scenario: Anomalies present

- **WHEN** anomalies exist in the database and the dashboard loads
- **THEN** an anomaly feed section displays the 10 most recent anomalies with
  severity badges (info/warn/error)

#### Scenario: No anomalies

- **WHEN** no anomalies exist
- **THEN** the anomaly feed section is not displayed or shows a "No anomalies
  detected" message

---

### Requirement: Anomaly retention

Anomaly pruning SHALL use a shorter default retention than request data. The
default SHALL be 30 days, configurable via `--anomaly-retention-days`.

#### Scenario: Anomaly retention shorter than data retention

- **WHEN** `--retention-days 365` is set and `--anomaly-retention-days` is not
- **THEN** anomalies older than 30 days are pruned while requests are kept for
  365 days

#### Scenario: Custom anomaly retention

- **WHEN** `--anomaly-retention-days 90` is set
- **THEN** anomalies older than 90 days are pruned
