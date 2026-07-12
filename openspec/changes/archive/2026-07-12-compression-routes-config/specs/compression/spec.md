## MODIFIED Requirements

### Requirement: Loopback compression processor

The system MAY transform request bodies through a configured local loopback
compression endpoint before upstream forwarding. Compression is configured
per-route via a `compression` block in the routes JSON configuration.
Compression SHALL be enabled when a route specifies `compression.endpoint`.
Eligible chat requests SHALL be transformed after routing and policy checks.
Only model and supported messages SHALL be sent to the processor; provider auth
and headers SHALL be excluded.

#### Scenario: Compression endpoint configured

- **WHEN** a route in the routes JSON includes
  `"compression": {"endpoint": "127.0.0.1:8787"}`
- **THEN** eligible POST `/chat/completions` requests on that route are sent to
  the compression endpoint before upstream forwarding

#### Scenario: Compression not configured

- **WHEN** no route has a `compression` block, or `compression.endpoint` is
  empty
- **THEN** requests are forwarded unchanged

#### Scenario: Ineligible request type

- **WHEN** a request is not a POST to `/chat/completions` or
  `/v1/chat/completions`
- **THEN** compression is skipped regardless of configuration

---

### Requirement: Compression configuration

Compression SHALL support optional strict mode, user-message scope control, and
optional target ratio, configured via the `compression` block in the routes
JSON.

#### Scenario: Strict mode enabled

- **WHEN** `"required": true` is set in the compression config and compression
  fails
- **THEN** the upstream request is blocked with a 502 error rather than
  forwarded uncompressed

#### Scenario: User message compression

- **WHEN** `"compress_user_messages": true` is set in the compression config
- **THEN** user message content is included in the compression request to the
  processor

#### Scenario: Target ratio

- **WHEN** `"target_ratio": 0.5` is set in the compression config
- **THEN** the compression processor is instructed to aim for a 50% compression
  ratio

---

### Requirement: Compression fail-open

When strict mode is NOT enabled, the proxy SHALL continue forwarding the
original request body if the compression processor is unreachable or returns an
error.

#### Scenario: Compression processor unreachable

- **WHEN** the compression endpoint is unreachable and `required` is false
- **THEN** the original request body is forwarded unchanged

#### Scenario: Compression processor returns error

- **WHEN** the compression endpoint returns a 4xx or 5xx and `required` is false
- **THEN** the original request body is forwarded unchanged

## REMOVED Requirements

### Requirement: Timeout configured

**Reason**: The compression timeout is now hardcoded to 30 seconds. Headroom is
a local loopback service; there is no practical need to tune the timeout.

**Migration**: Remove `--headroom-timeout` from your startup command. The
timeout is now always 30 seconds.
