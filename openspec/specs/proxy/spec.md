<!-- markdownlint-disable MD041 -->

## Purpose

Observe LLM API usage through a transparent local HTTP reverse proxy with
WebSocket support, health checks, graceful shutdown, and TTY-aware live
feedback.

## Requirements

### Requirement: Local single-user operation

The system SHALL run locally as a single-user tool with the dashboard embedded
in the binary. No hosted services are required.

#### Scenario: User starts the proxy

- **WHEN** user runs `copilot-monitor run --routes-config routes.json`
- **THEN** the proxy starts listening on 127.0.0.1:7733 and the embedded
  dashboard is available

---

### Requirement: Configurable provider proxying

The system SHALL proxy LLM API traffic for any provider defined in the route
configuration file. No provider routing SHALL be hardcoded.

#### Scenario: Route matches a configured path

- **WHEN** a request arrives at a path defined in the route config
- **THEN** the request is forwarded to the configured upstream host

#### Scenario: Route does not match any configured path

- **WHEN** a request arrives at a path not defined in the route config
- **THEN** the proxy returns a 502 error response

---

### Requirement: Transparent upstream forwarding

The system SHALL preserve normal upstream behavior by preserving method, path,
query, body, and required headers. Hop-by-hop headers SHALL be stripped.
Streaming responses SHALL be forwarded incrementally without buffering the
entire response.

#### Scenario: Streaming response is forwarded incrementally

- **WHEN** the upstream returns a streaming (chunked) response
- **THEN** each chunk is forwarded to the client as it arrives, with flush after
  each write

#### Scenario: Hop-by-hop headers are stripped

- **WHEN** a forwarded request contains Connection, Keep-Alive,
  Proxy-Authenticate, TE, Trailer, Transfer-Encoding, or Upgrade headers
- **THEN** those headers are removed before the request is sent upstream

---

### Requirement: WebSocket upgrade proxying

The system SHALL detect WebSocket upgrade requests and proxy them over TLS to
the upstream host bidirectionally. HTTP-level routing, policy checks, and model
matching SHALL apply before the upgrade.

#### Scenario: WebSocket upgrade is detected

- **WHEN** a request contains `Upgrade: websocket` and `Connection: upgrade`
  headers
- **THEN** the connection is hijacked, a TLS connection to the upstream is
  established, and data is relayed bidirectionally

#### Scenario: WebSocket headers are preserved

- **WHEN** proxying a WebSocket connection
- **THEN** all request headers including auth are cloned and sent to the
  upstream

---

### Requirement: WebSocket usage inspection

The system SHALL inspect proxied WebSocket text frames for Copilot Responses API
events and persist usage metadata.

#### Scenario: Usage data is extracted from response.completed events

- **WHEN** the upstream sends a WebSocket text frame containing a
  `response.completed` event with usage data
- **THEN** the usage metadata is persisted with the same fields as HTTP request
  rows

#### Scenario: Model name is tracked across WebSocket events

- **WHEN** a `response.create` event contains a model field
- **THEN** that model name is used for the eventual `response.completed`
  persistence

---

### Requirement: Log format flag

The `run` command SHALL support a `--log-format` flag accepting `human` and
`json`. The default SHALL be `human`. The `human` format SHALL produce rich
colored request output when stderr is a TTY, and plain aligned text when stderr
is not a TTY. The `json` format SHALL produce one JSON object per request,
identical to the previous default behavior.

#### Scenario: Default is human format

- **WHEN** `copilot-monitor run --routes-config routes.json` is executed without
  `--log-format`
- **THEN** the log format defaults to `human`

#### Scenario: Explicit JSON format

- **WHEN** `copilot-monitor run --routes-config routes.json --log-format json`
  is executed
- **THEN** each request is emitted as a single JSON object per line

#### Scenario: Invalid format value

- **WHEN** `copilot-monitor run --routes-config routes.json --log-format xml` is
  executed
- **THEN** the command exits with an error indicating valid values are `human`
  and `json`

---

### Requirement: Live session tail

The `run` command SHALL display a live session tail when stderr is a terminal,
updating every 2 seconds with the current session summary and replacing previous
output in-place. When the live tail is active, per-request log output (whether
human or JSON) SHALL be suppressed to avoid interleaving.

#### Scenario: Stderr is a TTY

- **WHEN** the proxy starts and stderr is a TTY and `--no-live` is not set
- **THEN** the live session tail activates, suppressing per-request log output
  to avoid interleaving

#### Scenario: Stderr is redirected or piped

- **WHEN** the proxy starts and stderr is not a TTY (redirected to file or pipe)
- **THEN** the live tail is disabled and per-request logs are emitted in the
  configured log format (human or json)

#### Scenario: Live tail is explicitly disabled

- **WHEN** the user passes `--no-live`
- **THEN** the live tail is disabled regardless of TTY status
- **AND** per-request logs are emitted in the configured log format (default:
  human)

#### Scenario: Live tail with JSON format

- **WHEN** the user passes `--log-format json` with live tail active
- **THEN** JSON per-request logs are still suppressed while the live tail is
  displayed

---

### Requirement: Health and ping endpoints

The system SHALL expose `/_health` returning JSON with status, uptime, request
count, and DB size, and `/_ping` returning OK for liveness checks.

#### Scenario: Health check succeeds

- **WHEN** a GET request is made to `/_health` and the store is reachable
- **THEN** the response is 200 with JSON
  `{"status":"ok","uptime_seconds":...,"requests_total":...,"db_size_bytes":...}`

#### Scenario: Health check when store is unreachable

- **WHEN** a GET request is made to `/_health` and the store is unreachable
- **THEN** the response is 503 with
  `{"status":"error","error":"store unreachable: ..."}`

#### Scenario: Ping check

- **WHEN** a GET request is made to `/_ping`
- **THEN** the response is 200 with body `OK`

---

### Requirement: Graceful shutdown

The system SHALL perform a graceful shutdown on SIGINT and SIGTERM, stopping new
connections and waiting up to 5 seconds for in-flight requests to complete.

#### Scenario: SIGINT received

- **WHEN** the proxy receives SIGINT
- **THEN** new connections are rejected, in-flight requests drain for up to 5
  seconds, and the process exits with code 130

#### Scenario: SIGTERM received

- **WHEN** the proxy receives SIGTERM
- **THEN** new connections are rejected, in-flight requests drain for up to 5
  seconds, and the process exits with code 0

---

### Requirement: Request identification

Every proxied request SHALL receive a unique, monotonic numeric identifier for
end-to-end log correlation.

#### Scenario: Request IDs increment

- **WHEN** the proxy handles consecutive requests
- **THEN** each request receives an ID one higher than the previous

---

### Requirement: Response buffer limits

SSE line buffers SHALL be capped at 1 MiB, full-JSON response buffers at 4 MiB,
and WebSocket frame payloads at 1 MiB. When a buffer exceeds its limit, it SHALL
be discarded and a parse error recorded.

#### Scenario: SSE buffer exceeds 1 MiB

- **WHEN** an SSE event line buffer grows beyond 1 MiB
- **THEN** the buffer is discarded and a parse error is recorded rather than
  growing unbounded

#### Scenario: JSON response buffer exceeds 4 MiB

- **WHEN** a full-JSON response buffer grows beyond 4 MiB
- **THEN** the buffer is discarded and a parse error is recorded

---

### Requirement: Startup validation

All fatal configuration errors SHALL be detected and reported before the proxy
accepts its first client connection.

#### Scenario: Invalid route config

- **WHEN** the route configuration is invalid
- **THEN** the process exits with code 1 before the HTTP listener starts

#### Scenario: Store cannot be opened

- **WHEN** the database cannot be opened
- **THEN** the process exits with code 1 before the HTTP listener starts

---

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
