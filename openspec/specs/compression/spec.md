<!-- markdownlint-disable MD041 -->

## Purpose

Optionally transform outgoing chat completion requests through a local loopback
compression processor to reduce token usage, with configurable strictness,
timeout, and detailed status tracking.

## Requirements

### Requirement: Loopback compression processor

The system MAY transform request bodies through a configured local loopback
compression endpoint before upstream forwarding. Eligible chat requests SHALL be
transformed after routing and policy checks. Only model and supported messages
SHALL be sent to the processor; provider auth and headers SHALL be excluded.

#### Scenario: Compression endpoint configured

- **WHEN** the proxy is started with
  `--headroom-url http://127.0.0.1:PORT/v1/compress`
- **THEN** eligible POST `/chat/completions` requests are sent to the
  compression endpoint before upstream forwarding

#### Scenario: Compression not configured

- **WHEN** no compression endpoint is configured
- **THEN** requests are forwarded unchanged

#### Scenario: Ineligible request type

- **WHEN** a request is not a POST to `/chat/completions` or
  `/v1/chat/completions`
- **THEN** compression is skipped regardless of configuration

---

### Requirement: Compression configuration

Compression SHALL support configurable timeout, optional strict mode,
user-message scope control, and optional target ratio.

#### Scenario: Timeout configured

- **WHEN** `--headroom-timeout 10s` is set
- **THEN** compression HTTP requests time out after 10 seconds

#### Scenario: Strict mode enabled

- **WHEN** `--headroom-required` is set and compression fails
- **THEN** the upstream request is blocked with a 502 error rather than
  forwarded uncompressed

#### Scenario: User message compression

- **WHEN** `--headroom-compress-user-messages` is set
- **THEN** user message content is included in the compression request to the
  processor

#### Scenario: Target ratio

- **WHEN** `--headroom-target-ratio 0.5` is set
- **THEN** the compression processor is instructed to aim for a 50% compression
  ratio

---

### Requirement: Compression fail-open

When strict mode is NOT enabled, the proxy SHALL continue forwarding the
original request body if the compression processor is unreachable or returns an
error.

#### Scenario: Compression processor unreachable

- **WHEN** the compression endpoint is unreachable and strict mode is off
- **THEN** the original request body is forwarded unchanged

#### Scenario: Compression processor returns error

- **WHEN** the compression endpoint returns a 4xx or 5xx and strict mode is off
- **THEN** the original request body is forwarded unchanged

---

### Requirement: Compression status labels

Compression SHALL emit stable status labels for every eligible request. Labels
distinguish: `applied` (tokens reduced), `no_change` (tokens unchanged),
`bypassed` (unsupported envelope), and `failed_*` with error categories.

#### Scenario: Tokens reduced

- **WHEN** compression reduces token count
- **THEN** status is `applied`

#### Scenario: No token change

- **WHEN** compression processes the request but no tokens are saved
- **THEN** status is `no_change`

#### Scenario: Unsupported envelope

- **WHEN** the compression processor does not support the request format
- **THEN** status is `bypassed`

#### Scenario: Compression fails in fail-open mode

- **WHEN** compression fails with a non-strict timeout and strict mode is off
- **THEN** status is `failed_fail_open` with a category indicating the error
  type

#### Scenario: Compression fails in strict mode

- **WHEN** compression fails with strict mode on
- **THEN** status is `failed_required` and the upstream request is not made

#### Scenario: Error categories

- **WHEN** a compression error occurs
- **THEN** the error is categorized as one of: canceled, timeout, http_4xx,
  http_5xx, invalid_response, or transport
