<!-- markdownlint-disable MD041 -->

## Purpose

Ensure that no prompts, completions, credentials, or sensitive content is
persisted by the proxy, and that all captured data remains local to the user's
machine.

## Requirements

### Requirement: No body or secret persistence

The system SHALL NOT persist prompts, completions, source code, auth headers,
cookies, or API keys in its own stores or logs by default.

#### Scenario: Request captured without body

- **WHEN** a request is proxied and persisted
- **THEN** the stored row contains metadata and token counts only; no prompt,
  completion, or source code text is stored

#### Scenario: Auth headers not logged

- **WHEN** structured logs are emitted for a proxied request
- **THEN** authorization, cookie, set-cookie, and any header containing
  token/secret/credential is redacted or omitted

---

### Requirement: In-memory body inspection

Request bodies MAY be inspected and transformed in memory for routing, policy,
forwarding, and local content processing such as compression. Request paths and
query strings SHALL be treated as potentially sensitive metadata.

#### Scenario: Body read for routing

- **WHEN** a request arrives
- **THEN** the body is read once into memory for model extraction, then restored
  for forwarding; the body content is not persisted

#### Scenario: Body sent to compression processor

- **WHEN** compression is configured and the request is eligible
- **THEN** only model and messages fields are sent to the compression processor;
  provider auth headers and cookies are excluded

---

### Requirement: Response body observation only

Response bodies SHALL be observed only to extract usage and model metadata.
Response content SHALL be forwarded to the client but not retained as body text.

#### Scenario: Streaming response observed

- **WHEN** the upstream returns a streaming response
- **THEN** each chunk is forwarded to the client; usage data is extracted from
  the stream but body text is not stored

---

### Requirement: Error body privacy

Upstream error bodies SHALL NOT be persisted or logged as previews.

#### Scenario: Upstream returns error

- **WHEN** the upstream returns a 4xx or 5xx response with an error body
- **THEN** the response is proxied to the client but the body text is not stored
  in any application data table or log

---

### Requirement: Debug output metadata-only

Optional debug output SHALL remain metadata-only with sensitive headers
redacted. Authorization, cookie, set-cookie, and headers containing token,
secret, or credential SHALL be redacted.

#### Scenario: Debug log emitted

- **WHEN** the usage debug logger is enabled and a request is proxied
- **THEN** the JSONL record contains response headers with sensitive fields
  replaced by `<redacted>`

---

### Requirement: Local data only

All captured data SHALL remain on the user's machine. No telemetry or uploads
SHALL be performed by default. The dashboard SHALL be embedded in the binary
with no external runtime dependencies.

#### Scenario: All data local

- **WHEN** the proxy captures and persists requests
- **THEN** data is stored in a local SQLite file only; no network requests
  transmit captured data to external services

---

### Requirement: Loopback binding

Local services SHALL bind to loopback addresses by default. Default proxy and
dashboard addresses SHALL use 127.0.0.1.

#### Scenario: Default proxy address

- **WHEN** the proxy starts without an explicit `--addr`
- **THEN** it listens on 127.0.0.1:7733

#### Scenario: Default dashboard address

- **WHEN** the serve command starts without an explicit `--addr`
- **THEN** it listens on 127.0.0.1:7734

---

### Requirement: Configurable database path

Users SHALL be able to choose the SQLite database path.

#### Scenario: Custom database path

- **WHEN** the proxy or a reporting command is started with
  `--db /custom/path/store.db`
- **THEN** captured data is read from and written to that path

#### Scenario: Default database path

- **WHEN** no `--db` is specified
- **THEN** the database is stored at `$XDG_DATA_HOME/copilot-monitor/store.db`
  or `~/.local/share/copilot-monitor/store.db`

---

### Requirement: No body fingerprints

The system SHALL avoid storing prompt-correlatable body fingerprints or treating
request paths and queries as safe identifiers. Derived compression metrics SHALL
be aggregate estimates, not body content.

#### Scenario: Compression metrics stored

- **WHEN** compression is applied
- **THEN** only estimated token counts and a status label are stored; no body
  hashes or transform details are persisted

---

### Requirement: Export privacy boundary

Exported data SHALL follow the same privacy boundary as persisted data: metadata
and token counts only, no bodies or secrets.

#### Scenario: CSV export

- **WHEN** data is exported to CSV
- **THEN** the output includes metadata columns (timestamp, endpoint, model,
  status, latency, token counts, project, compression metrics) and excludes
  bodies, prompts, completions, auth headers, and API keys
