## MODIFIED Requirements

### Requirement: Dashboard API

The system SHALL provide a read-only dashboard API with endpoints:
`/api/health`, `/api/stats`, `/api/cost`, `/api/today`, `/api/sessions`,
`/api/session/current`, `/api/stats/timeline`, `/api/export`, `/api/upstreams`,
`/api/policy`, `/api/policy/models`, `/api/anomalies`.

#### Scenario: All API endpoints respond

- **WHEN** the dashboard API is started
- **THEN** all listed endpoints return JSON responses with
  `Access-Control-Allow-Origin: *`
