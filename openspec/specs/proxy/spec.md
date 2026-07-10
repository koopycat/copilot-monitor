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

### Requirement: Live session tail

The `run` command SHALL display a live session tail when stderr is a terminal,
updating every 2 seconds with the current session summary and replacing previous
output in-place.

#### Scenario: Stderr is a TTY

- **WHEN** the proxy starts and stderr is a TTY and `--no-live` is not set
- **THEN** the live session tail activates, suppressing structured log output to
  avoid interleaving

#### Scenario: Stderr is redirected or piped

- **WHEN** the proxy starts and stderr is not a TTY (redirected to file or pipe)
- **THEN** the live tail is disabled and structured logs are emitted normally

#### Scenario: Live tail is explicitly disabled

- **WHEN** the user passes `--no-live`
- **THEN** the live tail is disabled regardless of TTY status

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
