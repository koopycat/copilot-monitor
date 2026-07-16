<!-- markdownlint-disable MD041 -->

## Purpose

Route all LLM API traffic to a single upstream host via the `--upstream` flag.
No route configuration file, provider prefixes, or per-path routing.

## Requirements

### Requirement: Single upstream flag

The system SHALL accept a required `--upstream` flag on the `run` command. All
incoming requests (except reserved local endpoints) SHALL be forwarded to the
specified upstream host. The upstream host SHALL be provided as a bare hostname
or host:port value; the proxy SHALL connect over HTTPS on port 443 unless a
custom port is included.

#### Scenario: Upstream flag provided

- **WHEN** the proxy starts with `--upstream api.githubcopilot.com`
- **THEN** all requests (except `/_ping` and `/_health`) are forwarded to
  `api.githubcopilot.com` over HTTPS

#### Scenario: Upstream with port

- **WHEN** the proxy starts with `--upstream localhost:8080`
- **THEN** all requests are forwarded to `localhost` on port `8080`

#### Scenario: Missing upstream flag

- **WHEN** `copilot-monitor run` is started without `--upstream`
- **THEN** the process exits with code 1 and a message indicating `--upstream`
  is required

---

### Requirement: All requests forwarded as-is

Every request (method, path, query, headers, body) SHALL be forwarded to the
configured upstream unchanged, except for standard hop-by-hop header stripping.
No prefix stripping, path rewriting, or provider label extraction SHALL be
performed.

#### Scenario: Request forwarded unchanged

- **WHEN** a POST request arrives at `/v1/chat/completions` with body
  `{"model":"gpt-4o"}`
- **THEN** the proxy forwards the same method, path, query, headers, and body to
  the upstream

#### Scenario: Hop-by-hop headers stripped

- **WHEN** a forwarded request contains Connection, Keep-Alive,
  Proxy-Authenticate, TE, Trailer, Transfer-Encoding, or Upgrade headers
- **THEN** those headers are removed before the request is sent upstream

---

### Requirement: Local endpoint reservation

The paths `/_ping` and `/_health` SHALL be handled locally by the proxy and
SHALL NOT be forwarded to the upstream. All other paths SHALL be forwarded.

#### Scenario: Ping handled locally

- **WHEN** a GET request is made to `/_ping`
- **THEN** the proxy returns 200 OK with body `OK` without contacting the
  upstream

#### Scenario: Health handled locally

- **WHEN** a GET request is made to `/_health`
- **THEN** the proxy returns a JSON health response without contacting the
  upstream

#### Scenario: Other paths forwarded

- **WHEN** a GET request is made to `/models`
- **THEN** the request is forwarded to the upstream

---

### Requirement: Unknown path handling

There are no "unknown" paths under the single-upstream model. Every path that is
not `/_ping` or `/_health` SHALL be forwarded. The upstream determines the
validity of the path and returns an appropriate response.

#### Scenario: Path unknown to upstream

- **WHEN** a request arrives at `/nonexistent` and the upstream returns 404
- **THEN** the 404 is forwarded back to the client as-is

---

### Requirement: WebSocket upgrade forwarding

WebSocket upgrade requests SHALL be detected and proxied over TLS to the
configured upstream host bidirectionally.

#### Scenario: WebSocket upgrade forwarded

- **WHEN** a request contains `Upgrade: websocket` and `Connection: upgrade`
  headers
- **THEN** the connection is hijacked, a TLS connection to the upstream is
  established, and data is relayed bidirectionally

#### Scenario: WebSocket headers preserved

- **WHEN** proxying a WebSocket connection
- **THEN** all request headers including auth are cloned and sent to the
  upstream
