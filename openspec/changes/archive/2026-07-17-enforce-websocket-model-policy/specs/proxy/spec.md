## MODIFIED Requirements

### Requirement: WebSocket upgrade proxying

The system SHALL detect WebSocket upgrade requests and proxy them over TLS to
the upstream host bidirectionally. The initial upgrade SHALL preserve headers,
and complete model-bearing client text messages SHALL be policy-checked before
they are forwarded after the upgrade.

#### Scenario: WebSocket upgrade is detected

- **WHEN** a request contains `Upgrade: websocket` and `Connection: upgrade`
  headers
- **THEN** the connection is hijacked, a TLS connection to the upstream is
  established, and data is relayed bidirectionally

#### Scenario: Model-bearing client message is checked

- **WHEN** the upgraded client sends a complete text message containing a model
- **THEN** the proxy evaluates the active model policy before writing that
  message to the upstream

#### Scenario: WebSocket headers are preserved

- **WHEN** proxying a WebSocket connection
- **THEN** all request headers including auth are cloned and sent to the
  upstream
