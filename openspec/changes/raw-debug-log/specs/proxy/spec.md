## ADDED Requirements

### Requirement: Raw debug logging

The `run` command SHALL support an optional `--raw-log <path>` flag that, when
set, writes one JSON record per proxied request to the specified file. The flag
SHALL only be valid with `run`, not `serve`.

#### Scenario: Raw log enabled

- **WHEN** the proxy is started with `--raw-log /tmp/raw.jsonl` and a request is
  proxied
- **THEN** a JSON record is appended to the file for that request

#### Scenario: Raw log disabled by default

- **WHEN** the proxy is started without `--raw-log`
- **THEN** no raw debug file is created or written

#### Scenario: Raw log on serve command

- **WHEN** `copilot-monitor serve --raw-log /tmp/raw.jsonl` is attempted
- **THEN** the command fails with an error indicating the flag is only valid
  with `run`

---

### Requirement: Raw log record content

Each raw log record SHALL include: request ID, timestamp, HTTP method, original
URL path, stripped provider prefix, resolved route endpoint, upstream host,
request model, request stream flag, request body (raw bytes, truncated to 1024
bytes), response status, response latency, redacted response headers, routing
decision, compression status, and policy decision.

#### Scenario: Full record written

- **WHEN** a request is proxied with raw logging enabled
- **THEN** the JSON record contains all required fields: `request_id`, `ts`,
  `method`, `path`, `provider`, `endpoint`, `upstream`, `model`, `stream`,
  `request_body` (base64-encoded, truncated to 1024 bytes), `status`,
  `latency_ms`, `response_headers` (redacted), `route_matched`,
  `compression_status`, `policy_allowed`

#### Scenario: Request body truncation

- **WHEN** a request body exceeds 1024 bytes
- **THEN** only the first 1024 bytes are logged and a
  `request_body_truncated: true` flag is set in the record

#### Scenario: Request body is empty

- **WHEN** a request has no body
- **THEN** `request_body` is set to an empty string

---

### Requirement: Raw log header redaction

Sensitive response headers SHALL be redacted in raw log records. Authorization,
cookie, set-cookie, and any header containing token, secret, or credential SHALL
be replaced with `<redacted>`.

#### Scenario: Authorization header redacted

- **WHEN** the upstream response includes an `Authorization` header
- **THEN** the raw log record shows `"authorization": "<redacted>"` for that
  header

#### Scenario: Cookie header redacted

- **WHEN** the upstream response includes `Set-Cookie` headers
- **THEN** the raw log record shows `"set-cookie": "<redacted>"` for those
  headers

---

### Requirement: Raw log privacy guard

A startup warning SHALL be printed when `--raw-log` is enabled, reminding the
user that raw request bodies may contain source code and prompts, and that the
log file should be treated as sensitive.

#### Scenario: Warning printed at startup

- **WHEN** the proxy starts with `--raw-log`
- **THEN** stderr receives a warning:
  `raw debug logging is enabled: request bodies (up to 1024 bytes) are written to <path>. This file may contain source code and prompts. Treat it as sensitive.`
