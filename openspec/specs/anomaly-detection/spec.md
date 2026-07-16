<!-- markdownlint-disable MD041 -->

## Purpose

Detect and persist operational anomalies in proxy traffic: SSE parse errors,
missing auth headers, unknown upstream hosts, unknown content types, and
unrecognized WebSocket events. Anomalies are written asynchronously through a
buffered channel with deduplication to avoid flooding.

## Requirements

### Requirement: Anomaly persistence

The system SHALL persist structured anomaly records in a dedicated `anomalies`
SQLite table. Each record SHALL include a category, severity level, request
reference, path, timestamp, and structured detail.

#### Scenario: Anomaly record is written

- **WHEN** an anomaly detection hook fires with category `parse_error` and
  severity `warn`
- **THEN** a row is inserted into the `anomalies` table with those values plus
  an ISO-8601 timestamp

#### Scenario: Anomaly table is created on start

- **WHEN** the proxy starts and the database is opened
- **THEN** the `anomalies` table exists (created with
  `CREATE TABLE IF NOT EXISTS`), causing no data loss on existing databases

### Requirement: SSE parse error detection

The system SHALL record an anomaly when the SSE observer fails to parse a
`data:` line or when a byte buffer exceeds its configured limit.

#### Scenario: SSE data line fails to parse

- **WHEN** the SSE observer receives a `data:` line containing malformed JSON
  and `json.Unmarshal` fails
- **THEN** an anomaly is recorded with category `parse_error`, severity `warn`,
  and the first 200 characters of the failing data line in the detail

#### Scenario: SSE buffer exceeds 1 MiB

- **WHEN** the SSE line buffer exceeds 1 MiB and is discarded
- **THEN** an anomaly is recorded with category `parse_error`, severity `warn`,
  and detail noting the buffer overflow

### Requirement: Auth header missing detection

The system SHALL record an anomaly when a request arrives at a recognized
Copilot CAPI path without an Authorization header. The anomaly record SHALL
include the model and upstream host.

#### Scenario: Chat completion without auth

- **WHEN** a request arrives at `/chat/completions` without an `Authorization`
  header
- **THEN** an anomaly is recorded with category `auth_missing`, severity
  `error`, the request path, the model from the request body, and the upstream
  host

#### Scenario: Health endpoint without auth

- **WHEN** a request arrives at `/_health` without an `Authorization` header
- **THEN** no anomaly is recorded (health endpoint does not require auth)

### Requirement: Unknown upstream host detection

The system SHALL record an anomaly when the proxy forwards a request to an
upstream host that has not been seen in any previous request within the current
proxy session.

#### Scenario: New upstream host appears

- **WHEN** the proxy forwards to an upstream host it has never proxied to before
  in the current session
- **THEN** an anomaly is recorded with category `unknown_upstream`, severity
  `info`, and the upstream hostname in the detail

### Requirement: Unknown Content-Type detection

The system SHALL record an anomaly when a response arrives with a Content-Type
header that is neither `text/event-stream`, `application/json`, nor
`text/plain`.

#### Scenario: Unknown Content-Type on request

- **WHEN** a response arrives with Content-Type `application/octet-stream`
- **THEN** an anomaly is recorded with category `unknown_content_type`, severity
  `info`, and the Content-Type value in the detail

#### Scenario: Known Content-Type is not recorded

- **WHEN** a response arrives with Content-Type `text/event-stream`
- **THEN** no anomaly is recorded for unknown Content-Type

### Requirement: WebSocket unknown event type detection

The system SHALL record an anomaly when a WebSocket text frame contains a JSON
event whose `type` field is not in the set of recognized Copilot Responses API
event types.

#### Scenario: Unknown event type in WebSocket

- **WHEN** a WebSocket text frame contains `{"type": "response.streaming", ...}`
  and `response.streaming` is not in the recognized set
- **THEN** an anomaly is recorded with category `unknown_ws_event`, severity
  `info`, and the event type in the detail

### Requirement: Anomaly deduplication

The system SHALL suppress duplicate anomaly records for the same (category,
path, detail_hash) tuple within a 5-minute cooldown window to prevent flooding.

#### Scenario: Duplicate anomaly suppressed within cooldown

- **WHEN** an anomaly is recorded for category `auth_missing` and path
  `/v1/chat/completions`
- **AND** a second anomaly for the same category and path arrives 2 minutes
  later
- **THEN** no additional row is written to the `anomalies` table

#### Scenario: Anomaly re-recorded after cooldown

- **WHEN** an anomaly was recorded for category `auth_missing` and path
  `/v1/chat/completions` 6 minutes ago
- **AND** a new anomaly for the same category and path arrives
- **THEN** a new row is written to the `anomalies` table

### Requirement: Anomaly write non-blocking behavior

The system SHALL write anomaly records through a buffered channel with a
dedicated goroutine, ensuring anomaly recording never blocks the proxy's
response path.

#### Scenario: Buffer is not full

- **WHEN** an anomaly is recorded and the channel buffer has capacity
- **THEN** the record is sent to the channel and the calling goroutine returns
  immediately without waiting for the database write

#### Scenario: Buffer is full

- **WHEN** an anomaly is recorded and the channel buffer is full
- **THEN** the record is dropped, an internal dropped counter is incremented,
  and the calling goroutine returns immediately
