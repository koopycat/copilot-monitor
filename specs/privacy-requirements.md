# Privacy Requirements

These requirements are normative. Changes that affect capture, persistence,
debugging, routing, or export behavior must preserve them unless the
requirements are deliberately revised.

## Data Minimization

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PRIV-001 | The product must not persist prompts, completions, source code, auth headers, cookies, or API keys by default. | Stored request rows contain metadata and token counts only. |
| PRIV-002 | Request bodies may be inspected only to extract safe metadata needed for routing/reporting. | Extracted request metadata is limited to fields such as model and stream mode. |
| PRIV-003 | Response bodies may be observed only to extract usage and model metadata. | Response content is forwarded to the client but not retained as body text. |
| PRIV-004 | Upstream error bodies must not be persisted or logged as previews. | Error responses may be proxied to the client, but body text is not stored as application data. |
| PRIV-005 | Optional debug output must remain metadata-only. | Debug records may include safe response headers and usage-detection state; sensitive headers are redacted. |

## Locality And Exposure

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PRIV-006 | All captured data must remain on the user's machine. | No telemetry or uploads are performed by default. |
| PRIV-007 | Local services must bind to loopback addresses by default. | Default proxy and dashboard addresses use `127.0.0.1`. |
| PRIV-008 | Users must be able to choose the SQLite database path. | Commands that read or write captured data accept a database path override. |

## Sensitive Derived Data

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PRIV-009 | The product must avoid storing prompt-correlatable body fingerprints without a product use. | Request body hashes are not stored by default. |
| PRIV-010 | Exported data must follow the same privacy boundary as persisted data. | Exports include metadata and token counts, not bodies or secrets. |

## Privacy Review Triggers

Any change requires explicit privacy review when it:

- adds a persisted column,
- changes request or response capture behavior,
- expands debug logging,
- changes export fields,
- exposes services beyond loopback defaults,
- stores derived identifiers from request or response content.
