## ADDED Requirements

### Requirement: WebSocket model policy enforcement

The system SHALL evaluate the active model policy before forwarding a complete
client-to-upstream WebSocket text message that explicitly contains a model. A
disallowed model message SHALL not be forwarded upstream.

#### Scenario: Blocked WebSocket model

- **WHEN** a client sends a complete WebSocket text message naming a model
  blocked by the active policy
- **THEN** the proxy does not forward that message to the upstream
- **AND** persists a blocked request with status 403 and `stream=true`
- **AND** sends the client a WebSocket close frame with code 1008 and reason
  `model_blocked`

#### Scenario: Allowed WebSocket model

- **WHEN** a client sends a complete WebSocket text message naming a model
  allowed by the active policy
- **THEN** the proxy forwards the original frame sequence upstream

#### Scenario: WebSocket message without a usable model

- **WHEN** a client text message has no model, cannot be parsed as JSON, or
  exceeds the bounded inspection buffer
- **THEN** the proxy forwards it unchanged according to fail-open policy
  semantics
