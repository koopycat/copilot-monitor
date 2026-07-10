# Privacy Requirements

These requirements are normative. Changes that affect capture, persistence,
debugging, routing, request transformation, or export behavior must preserve
them unless the requirements are deliberately revised.

## Data Minimization

| ID       | Requirement                                                                                                                                                                                                                     | Acceptance Criteria                                                                                                                                                                                                                                                                                                                               |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PRIV-001 | The product must not persist prompts, completions, source code, auth headers, cookies, or API keys in its own stores or logs by default.                                                                                        | Product-owned request rows and logs contain metadata and token counts only; no requirement depends on a persisted body-text table. Separately operated local processors manage any content retention under their own configuration.                                                                                                               |
| PRIV-002 | Request bodies may be inspected and transformed in memory for routing, policy, forwarding, and local content processing such as compression. Request paths and query strings must be treated as potentially sensitive metadata. | Full request content may be sent to a configured loopback processor or the selected upstream provider. Product-owned stores and logs do not retain body text, and provider credentials, cookies, and unrelated headers are not sent to content-only processors. No separate privacy consent gate is required for this personal, single-user tool. |
| PRIV-003 | Response bodies may be observed only to extract usage and model metadata.                                                                                                                                                       | Response content is forwarded to the client but not retained as body text.                                                                                                                                                                                                                                                                        |
| PRIV-004 | Upstream error bodies must not be persisted or logged as previews.                                                                                                                                                              | Error responses may be proxied to the client, but body text is not stored as application data.                                                                                                                                                                                                                                                    |
| PRIV-005 | Optional debug output must remain metadata-only.                                                                                                                                                                                | Debug records may include safe response headers and usage-detection state; sensitive headers are redacted.                                                                                                                                                                                                                                        |

## Locality And Exposure

| ID       | Requirement                                                | Acceptance Criteria                                                                                                                                      |
| -------- | ---------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PRIV-006 | All captured data must remain on the user's machine.       | No telemetry or uploads are performed by default; any browser-dashboard request for a hosted runtime must not include captured data.                     |
| PRIV-007 | Local services must bind to loopback addresses by default. | Default proxy and dashboard addresses use `127.0.0.1`, and browser-accessible JSON endpoints remain within the same privacy boundary as the local store. |
| PRIV-008 | Users must be able to choose the SQLite database path.     | Commands that read or write captured data accept a database path override.                                                                               |

## Sensitive Derived Data

| ID       | Requirement                                                                                                                     | Acceptance Criteria                                                                                                                                                                                                                                                  |
| -------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PRIV-009 | The product must avoid storing prompt-correlatable body fingerprints or treating request paths and queries as safe identifiers. | Request body hashes are not stored by default, and path/query values are retained only when needed for routing/reporting. Derived compression metrics (token counts, status, latency) are aggregate estimates, not body content, and do not constitute fingerprints. |
| PRIV-010 | Exported data must follow the same privacy boundary as persisted data.                                                          | Exports include metadata and token counts, not bodies or secrets.                                                                                                                                                                                                    |

## Privacy Review Triggers

Any change requires explicit privacy review when it:

- adds a persisted column,
- changes request or response capture behavior,
- changes which local processors receive request content,
- expands debug logging,
- changes export fields,
- exposes services beyond loopback defaults,
- stores derived identifiers from request or response content.

### Reviewed: compression metadata columns (2026-07-10)

Added `compression_status`, `compression_original_tokens`,
`compression_final_tokens`, `compression_latency_ms` as nullable columns on the
`requests` table. These store estimated aggregate token counts and a stable
status label. No body text, hashes, transform details, or content identifiers
are persisted. Provider response usage remains authoritative for cost.

Exported rows include the same four fields. Dashboard stats include derived
aggregates (`compression_removed_tokens`, `avg_compression_ratio`) computed in
SQL. No dollar savings are calculated from compression estimates.
