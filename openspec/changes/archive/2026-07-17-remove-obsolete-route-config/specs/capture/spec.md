## MODIFIED Requirements

### Requirement: Zero-token persistence

The system SHALL persist proxied requests even when no token usage is present.
Rows without usage SHALL have zero token counts and a `usage_missing` flag.

#### Scenario: Usage field absent from response

- **WHEN** a proxied response contains no usage data
- **THEN** the request is persisted with zero token values and `usage_missing`
  set to true

---

### Requirement: Metadata capture without usage

The system SHALL persist request metadata for proxied requests that do not
expose token usage.

#### Scenario: Response without usage

- **WHEN** a proxied response does not contain token usage
- **THEN** request metadata (endpoint, method, path, model, status, latency) is
  persisted without token counts
