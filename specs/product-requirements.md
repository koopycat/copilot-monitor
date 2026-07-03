# Product Requirements

These requirements define what Copilot Monitor must provide. They avoid
implementation-specific paths, package names, and implementation plans.

## Scope

Copilot Monitor is a single-user, local developer utility for observing GitHub
Copilot model usage. It provides a transparent local proxy, CLI reports, and a
local dashboard over captured metadata and token counts. Proxying, capture,
persistence, CLI reporting, and dashboard API behavior are local-first; the
current browser dashboard may use a documented hosted runtime dependency.

## Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PROD-001 | The product must run locally as a single-user tool. | A user can start the proxy and dashboard API without hosted services; any browser-dashboard hosted runtime dependency is documented. |
| PROD-002 | The product must proxy supported GitHub Copilot model API traffic. | Supported Copilot chat, agent, completion, message, embedding, model metadata, ping, and WebSocket paths have defined routing behavior. |
| PROD-003 | The proxy must preserve normal Copilot behavior for the client. | Forwarded requests preserve method, path, query, body, and required headers; streaming responses are forwarded incrementally. |
| PROD-004 | The product must capture usage metadata for supported model-generation traffic when token usage is present. | Captured rows include timestamp, endpoint, model when known, status, latency, project label when supplied, and token counts. When both request and response metadata expose a model, the request model takes precedence. Token usage can be normalized from OpenAI-style or Anthropic-style usage fields, including prompt/input, cached/cache read, cache write, completion/output, and total counts. Streaming and non-streaming responses are both tolerated when usage appears. |
| PROD-005 | The product must support metadata-only capture for endpoints that are useful for reporting but do not expose token usage. | Metadata-only endpoints can be stored without prompt or completion text. |
| PROD-006 | The product must provide read-only reporting through CLI commands. | Users can inspect stats, cost, today, sessions, compare, and export outputs; commands may rebuild derived session summaries before reporting, but they do not alter captured request rows. |
| PROD-007 | The product must provide a local read-only dashboard API. | Users can start a local dashboard service and query health, stats, cost, sessions, timeline, export, compare, and current-session endpoints; session views may refresh derived session summaries before responding, but captured request rows remain unchanged. |
| PROD-008 | Cost output must be labeled as an estimate, not as an actual bill. | CLI and dashboard cost displays communicate estimated AI-credit list-price semantics, identify fallback pricing when used, and mark not-billed rows as zero-cost. |
| PROD-009 | Export must provide portable reporting data for captured request rows. | CSV export includes the stored metadata fields used by reports for rows that meet the export filters, and omits bodies and secrets. |
| PROD-010 | Sessions must group activity using an inactivity gap. | Session reports group requests using a 30-minute inactivity threshold. |
| PROD-011 | The product must expose a derived current-session view. | The current-session view reflects the most recent derived session and can indicate whether it is active or idle using the same inactivity gap. |

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
| REPORT-003 | Machine-readable JSON output must be available for supported CLI report commands. |
| REPORT-004 | Dashboard data endpoints must be read-only with respect to captured requests and may refresh derived session summaries when needed. |

## Quality Requirements

| ID | Requirement |
|---|---|
| QUAL-001 | The proxy must not buffer an entire streaming response before forwarding it. |
| QUAL-002 | Ordinary development changes should be verifiable with the test suite. |
| QUAL-003 | The local dashboard should remain dependency-light and require no build step. |
