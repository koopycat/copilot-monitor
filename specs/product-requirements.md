# Product Requirements

These requirements define what llm-proxy must provide. They avoid
implementation-specific paths, package names, and implementation plans.

## Scope

llm-proxy is a single-user, local developer utility for observing LLM API usage
through a transparent local HTTP proxy. It captures metadata and token counts
from proxied requests and surfaces them through CLI reports and a local
dashboard. All routing is configuration-driven — no provider is hardcoded or
privileged.

Proxying, capture, persistence, CLI reporting, and dashboard API behavior are
local-first; the current browser dashboard may use a documented hosted runtime
dependency.

## Functional Requirements

| ID       | Requirement                                                                                                               | Acceptance Criteria                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| -------- | ------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PROD-001 | The product must run locally as a single-user tool.                                                                       | A user can start the proxy and dashboard API without hosted services; any browser-dashboard hosted runtime dependency is documented.                                                                                                                                                                                                                                                                                                                                                                |
| PROD-002 | The product must proxy LLM API traffic for any provider defined in the route configuration.                               | Users supply a JSON route config file mapping paths to upstream hosts, capture modes, and optional model filters. All recognized paths are forwarded; unknown paths are rejected. No provider routing is hardcoded.                                                                                                                                                                                                                                                                                 |
| PROD-003 | The proxy must preserve normal upstream behavior for the client.                                                          | Forwarded requests preserve method, path, query, body, and required headers; hop-by-hop headers are stripped; streaming responses are forwarded incrementally.                                                                                                                                                                                                                                                                                                                                      |
| PROD-004 | The product must capture usage metadata for model-generation traffic when token usage is present.                         | Captured rows include timestamp, endpoint label, model when known, status, latency, project label when supplied, provider, and token counts. When both request and response metadata expose a model, the request model takes precedence. Token usage can be normalized from OpenAI-style or Anthropic-style usage fields, including prompt/input, cached/cache read, cache write, completion/output, and total counts. Streaming and non-streaming responses are both tolerated when usage appears. |
| PROD-005 | The product must persist requests even when no token usage is present.                                                    | Requests routed with `capture: "usage"` are persisted with zero token counts and a `usage_missing` flag when the upstream response contains no usage data.                                                                                                                                                                                                                                                                                                                                          |
| PROD-006 | The product must support metadata-only capture for endpoints that are useful for reporting but do not expose token usage. | Metadata-only endpoints can be stored without prompt or completion text.                                                                                                                                                                                                                                                                                                                                                                                                                            |
| PROD-007 | The product must provide read-only reporting through CLI commands.                                                        | Users can inspect stats, cost, today, sessions, and export outputs; commands may rebuild derived session summaries before reporting, but they do not alter captured request rows.                                                                                                                                                                                                                                                                                                                   |
| PROD-008 | The product must provide a local read-only dashboard API.                                                                 | Users can start a local dashboard service and query health, stats, cost, sessions, timeline, export, and current-session endpoints; session views may refresh derived session summaries before responding, but captured request rows remain unchanged.                                                                                                                                                                                                                                              |
| PROD-009 | Cost output must be labeled as an estimate, not as an actual bill.                                                        | CLI and dashboard cost displays communicate estimated list-price semantics, identify fallback pricing when used, and mark not-billed rows as zero-cost.                                                                                                                                                                                                                                                                                                                                             |
| PROD-010 | Export must provide portable reporting data for captured request rows.                                                    | CSV export includes the stored metadata fields used by reports for rows that meet the export filters, and omits bodies and secrets.                                                                                                                                                                                                                                                                                                                                                                 |
| PROD-011 | Sessions must group activity using an inactivity gap.                                                                     | Session reports group requests using a 30-minute inactivity threshold.                                                                                                                                                                                                                                                                                                                                                                                                                              |
| PROD-012 | The product must expose a derived current-session view.                                                                   | The current-session view reflects the most recent derived session and can indicate whether it is active or idle using the same inactivity gap.                                                                                                                                                                                                                                                                                                                                                      |
| PROD-013 | The product must allow users to block specific AI models from being used through configured providers.                    | Users can set policy mode and model list through API; blocked models return 403; allowed models pass through normally.                                                                                                                                                                                                                                                                                                                                                                              |
| PROD-014 | The product may transform request bodies through a configured local loopback processor before upstream forwarding.        | When a loopback compression endpoint is configured, eligible chat requests are transformed after routing and policy checks; only model and supported messages are sent to the processor; provider auth and headers are excluded.                                                                                                                                                                                                                                                                    |
| PROD-015 | The product must persist estimated compression metrics when compression is applied.                                       | Compression status, estimated original tokens, estimated final tokens, and compression latency are stored as nullable columns; derived aggregates (tokens removed, ratio) appear in stats and export; provider usage remains authoritative for cost.                                                                                                                                                                                                                                                |

## Routing Requirements

| ID        | Requirement                                                                                                 |
| --------- | ----------------------------------------------------------------------------------------------------------- |
| ROUTE-001 | Routing must be defined entirely through a JSON route configuration file. No provider routing is hardcoded. |
| ROUTE-002 | `--routes-config` is required for the `run` command. Starting without it must fail with a clear error.      |
| ROUTE-003 | Unknown inbound paths must be rejected with an error response.                                              |
| ROUTE-004 | Local health and ping paths must not be forwarded upstream.                                                 |
| ROUTE-005 | Capture mode must be explicit for every configured route.                                                   |
| ROUTE-006 | Routes support optional model-based filtering with exact match and `*`-prefix patterns.                     |
| ROUTE-007 | Routes support an explicit `provider` label used in reports and cost attribution.                           |
| ROUTE-008 | Routes support a `not_billed` flag that marks rows as zero-cost regardless of tokens.                       |
| ROUTE-009 | Known provider prefixes in URL paths (`/copilot/...`, `/openai/...`) are stripped before route matching.    |

## Route Configuration Schema

The route config is a JSON file (with optional `//` comments) containing a
`routes` array. Each route has these fields:

| Field                  | Required | Description                                                      |
| ---------------------- | -------- | ---------------------------------------------------------------- |
| `path`                 | Yes      | URL path pattern; must start with `/`.                           |
| `upstream_host`        | Yes\*    | Host to forward requests to (\*empty for `capture: "local"`).    |
| `capture`              | Yes      | One of: `usage`, `metadata`, `none`, `tunnel`, `local`.          |
| `label`                | No       | Human-readable endpoint name for reports (defaults to the path). |
| `prefix_match`         | No       | If true, matches all paths with this prefix.                     |
| `models`               | No       | Array of model patterns to filter; empty/null matches any model. |
| `provider`             | No       | Provider name for attribution in reports and cost lookups.       |
| `not_billed`           | No       | If true, marks this endpoint as zero-cost regardless of tokens.  |
| `upstream_path_prefix` | No       | Prepend this prefix to the path before forwarding upstream.      |

## Policy Requirements

| ID      | Requirement                                                                                                                     |
| ------- | ------------------------------------------------------------------------------------------------------------------------------- |
| POL-001 | The proxy must support a global model allow/block policy.                                                                       |
| POL-002 | Default behavior with no policy configured must allow all models.                                                               |
| POL-003 | Blocked models must return HTTP 403 with a JSON error body identifying the blocked model.                                       |
| POL-004 | Blocked attempts must be persisted to the requests table with status 403 and zero token counts.                                 |
| POL-005 | Policy must support three modes: allow_all, blocklist, and allowlist.                                                           |
| POL-006 | Model patterns must support `*` suffix for prefix matching (e.g., `gpt-*`).                                                     |
| POL-007 | The policy must be manageable through read-only dashboard API endpoints.                                                        |
| POL-008 | The policy must be updatable through dashboard API endpoints.                                                                   |
| POL-009 | Policy evaluation must fail open: unknown modes, nil policy, empty model, and store errors all default to allowing the request. |
| POL-010 | Model discovery endpoint must return all unique model names from captured request history.                                      |

## CLI Requirements

| ID      | Requirement                                                                                                         |
| ------- | ------------------------------------------------------------------------------------------------------------------- |
| CLI-001 | A `validate` subcommand must check a route config file for errors without starting the proxy.                       |
| CLI-002 | An `init` subcommand must create a starter route config file with auto-detected providers.                          |
| CLI-003 | `init` must refuse to overwrite an existing config without `--force`.                                               |
| CLI-004 | The `run` command must emit a one-line startup banner showing the address, route count, and a verification command. |
| CLI-005 | The `run` command must support structured JSON logging (default) and human-readable logging via `--log-format`.     |
| CLI-006 | All reporting commands must produce valid JSON when `--json` is specified, including empty result sets.             |
| CLI-007 | Reporting commands must display a footnote when requests with `usage_missing` exist in the result set.              |
| CLI-008 | Exit codes follow convention: 0 = success, 1 = runtime error, 2 = usage error, 130 = SIGINT.                        |

## Health Endpoint

| ID       | Requirement                                                                                        |
| -------- | -------------------------------------------------------------------------------------------------- |
| HLTH-001 | `GET /_health` on the proxy port must return JSON with status, uptime, request count, and DB size. |
| HLTH-002 | `/_health` must return 503 if the store is unreachable.                                            |
| HLTH-003 | `/_ping` must return `OK` for basic liveness checks.                                               |

## Reporting Requirements

| ID         | Requirement                                                                                                                         |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| REPORT-001 | Reports must support filtering by time range where applicable.                                                                      |
| REPORT-002 | Reports must support project filtering where applicable.                                                                            |
| REPORT-003 | Machine-readable JSON output must be available for supported CLI report commands.                                                   |
| REPORT-004 | Dashboard data endpoints must be read-only with respect to captured requests and may refresh derived session summaries when needed. |
| REPORT-005 | Reports must support filtering by provider.                                                                                         |

## Quality Requirements

| ID       | Requirement                                                                   |
| -------- | ----------------------------------------------------------------------------- |
| QUAL-001 | The proxy must not buffer an entire streaming response before forwarding it.  |
| QUAL-002 | Ordinary development changes should be verifiable with the test suite.        |
| QUAL-003 | The local dashboard should remain dependency-light and require no build step. |
