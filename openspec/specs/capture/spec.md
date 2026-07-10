<!-- markdownlint-disable MD041 -->

## Purpose

Extract usage metadata and token counts from proxied LLM API responses, persist
them to a local SQLite store, group requests into sessions, and emit structured
logs and debug output.

## Requirements

### Requirement: Usage metadata capture

The system SHALL capture usage metadata for model-generation traffic when token
usage is present. Captured rows SHALL include timestamp, endpoint label, model
(when known), streaming flag, status, latency, project label, provider, and
token counts.

#### Scenario: OpenAI-style usage captured

- **WHEN** the upstream response contains `usage.prompt_tokens` and
  `usage.completion_tokens`
- **THEN** prompt and completion token counts are extracted and persisted

#### Scenario: Anthropic-style usage captured

- **WHEN** the upstream response contains `usage.input_tokens` and
  `usage.output_tokens`
- **THEN** input tokens are mapped to prompt_tokens and output tokens to
  completion_tokens

#### Scenario: Cache read tokens captured

- **WHEN** the response contains `usage.cached_input_tokens` (OpenAI) or
  `usage.cache_read_input_tokens` (Anthropic)
- **THEN** cache read token count is extracted and persisted

#### Scenario: Cache write tokens captured

- **WHEN** the response contains `usage.cache_write_tokens` (OpenAI) or
  `usage.cache_creation_input_tokens` (Anthropic)
- **THEN** cache write token count is extracted and persisted

#### Scenario: Request model takes precedence

- **WHEN** both the request body and the response indicate a model name
- **THEN** the request model is used as the authoritative model for the captured
  row

#### Scenario: Model extracted from nested Copilot API response

- **WHEN** the request body contains `response.model` (Copilot Responses API
  format)
- **THEN** that nested model name is extracted

---

### Requirement: Zero-token persistence

The system SHALL persist requests even when no token usage is present. Requests
routed with `capture: "usage"` SHALL be persisted with zero token counts and a
`usage_missing` flag.

#### Scenario: Usage field absent from response

- **WHEN** a response to a route with `capture: "usage"` contains no usage data
- **THEN** the request is persisted with zero token values and `usage_missing`
  set to true

---

### Requirement: Metadata-only capture

The system SHALL support metadata-only capture for endpoints that are useful for
reporting but do not expose token usage.

#### Scenario: Metadata-only endpoint

- **WHEN** a route has `capture: "metadata"`
- **THEN** request metadata (endpoint, method, path, model, status, latency) is
  persisted without requiring usage data

---

### Requirement: Session grouping

The system SHALL group requests into sessions using a 30-minute inactivity
threshold.

#### Scenario: Consecutive requests within threshold

- **WHEN** two consecutive requests for the same project occur within 30 minutes
  of each other
- **THEN** they are grouped into the same session

#### Scenario: Gap exceeds threshold

- **WHEN** two consecutive requests occur more than 30 minutes apart
- **THEN** they are placed in separate sessions

---

### Requirement: Current session view

The system SHALL expose a derived current-session view that reflects the most
recent session and indicates whether it is active or idle.

#### Scenario: Active session

- **WHEN** the last request in the most recent session occurred within the last
  30 minutes
- **THEN** the session status is "active"

#### Scenario: Idle session

- **WHEN** the last request in the most recent session occurred more than 30
  minutes ago
- **THEN** the session status is "idle" with an indication of how long ago it
  ended

---

### Requirement: Compression metrics persistence

The system SHALL persist estimated compression metrics when compression is
applied: compression status, estimated original tokens, estimated final tokens,
and compression latency as nullable columns. Derived aggregates (tokens removed,
ratio) SHALL appear in stats and export.

#### Scenario: Compression applied to request

- **WHEN** compression is applied to a request
- **THEN** compression status, original tokens, final tokens, and latency are
  persisted with the row

#### Scenario: Compression not attempted

- **WHEN** compression is not configured or the request is not eligible
- **THEN** compression columns are stored as null

---

### Requirement: Usage debug logging

The system MAY emit metadata-only JSONL debug logs for usage-detection
troubleshooting. Debug records SHALL include request ID, endpoint, request
model, response model, usage-detection status, and safe response headers.
Sensitive headers (authorization, cookie, set-cookie, and headers containing
token, secret, or credential) SHALL be redacted.

#### Scenario: Debug log enabled

- **WHEN** the proxy is started with `--usage-debug-log path/to/debug.jsonl`
- **THEN** each proxied request writes a JSON record to the debug file with
  redacted sensitive headers

---

### Requirement: Structured request logging

The system SHALL emit structured log entries for every proxied request
containing the request ID, method, path, upstream host, model, status code,
latency, capture mode, and whether token usage was detected. Log entries SHALL
omit body text and credentials.

#### Scenario: Successful request logged

- **WHEN** a request is proxied and completed
- **THEN** a structured log entry is emitted with all required fields

#### Scenario: Log format

- **WHEN** `--log-format json` is set (default)
- **THEN** log entries are emitted as JSON objects
- **WHEN** `--log-format human` is set
- **THEN** log entries are emitted as human-readable text
