# Product Requirements

These requirements define what Copilot Monitor must provide. They avoid
implementation-specific paths, package names, and implementation plans.

## Scope

Copilot Monitor is a single-user, local developer utility for observing GitHub
Copilot model usage. It provides a transparent local proxy, CLI reports, and a
local dashboard over captured metadata and token counts.

## Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PROD-001 | The product must run locally as a single-user tool. | A user can start the proxy and dashboard without hosted services. |
| PROD-002 | The product must proxy supported GitHub Copilot model API traffic. | Supported Copilot chat, agent, completion, message, embedding, model metadata, ping, and WebSocket paths have defined routing behavior. |
| PROD-003 | The proxy must preserve normal Copilot behavior for the client. | Forwarded requests preserve method, path, query, body, and required headers; streaming responses are forwarded incrementally. |
| PROD-004 | The product must capture usage metadata for supported model-generation traffic when token usage is present. | Captured rows include timestamp, endpoint, model when known, status, latency, project label when supplied, and token counts. |
| PROD-005 | The product must support metadata-only capture for endpoints that do not expose token usage but are useful for reporting. | Metadata-only endpoints can be stored without prompt or completion text. |
| PROD-006 | The product must provide read-only reporting through CLI commands. | Users can inspect stats, cost, today, sessions, compare, and export outputs. |
| PROD-007 | The product must provide a local read-only dashboard API. | Users can start a local dashboard service and query health, stats, cost, sessions, timeline, export, compare, and current-session endpoints. |
| PROD-008 | Cost output must be labeled as an estimate, not as an actual bill. | CLI and dashboard cost displays communicate estimated AI-credit list-price semantics. |
| PROD-009 | Export must provide portable captured request metadata. | CSV export includes timestamp, endpoint, model, status, latency, token counts, and project. |
| PROD-010 | Sessions must group activity using an inactivity gap. | Session reports group requests using a 30-minute inactivity threshold. |

## Routing Requirements

| ID | Requirement |
|---|---|
| ROUTE-001 | Unknown inbound paths must fail closed instead of silently proxying. |
| ROUTE-002 | Local health/ping traffic must not be forwarded upstream. |
| ROUTE-003 | Model metadata routes must not persist request rows. |
| ROUTE-004 | WebSocket response traffic must be tunneled without usage persistence. |
| ROUTE-005 | Capture mode must be explicit for every supported route. |

## Reporting Requirements

| ID | Requirement |
|---|---|
| REPORT-001 | Reports must support filtering by time range where applicable. |
| REPORT-002 | Reports must support project filtering where applicable. |
| REPORT-003 | Machine-readable JSON output must be available for primary CLI reports. |
| REPORT-004 | Dashboard data endpoints must be read-only. |

## Quality Requirements

| ID | Requirement |
|---|---|
| QUAL-001 | The proxy must not buffer an entire streaming response before forwarding it. |
| QUAL-002 | Ordinary development changes should be verifiable with the test suite. |
| QUAL-003 | The local dashboard should remain dependency-light and require no build step. |
