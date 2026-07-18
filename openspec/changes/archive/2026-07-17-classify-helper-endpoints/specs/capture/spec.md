## MODIFIED Requirements

### Requirement: Usage metadata capture

The system SHALL capture usage metadata for model-generation traffic when token
usage is present. Captured rows SHALL include timestamp, endpoint label,
endpoint kind (`inference` or `control_plane`), model (when known), streaming
flag, status, latency, project label, provider, and token counts.

#### Scenario: OpenAI-style usage captured

- **WHEN** the upstream response contains `usage.prompt_tokens` and
  `usage.completion_tokens`
- **THEN** prompt and completion token counts are extracted and persisted

#### Scenario: Anthropic-style usage captured

- **WHEN** the upstream response contains `usage.input_tokens` and
  `usage.output_tokens`
- **THEN** input tokens are mapped to prompt_tokens and output tokens to
  completion_tokens

#### Scenario: Cache read tokens captured

- **WHEN** the response contains `usage.cached_input_tokens` (OpenAI) or
  `usage.cache_read_input_tokens` (Anthropic)
- **THEN** cache read token count is extracted and persisted

#### Scenario: Cache write tokens captured

- **WHEN** the response contains `usage.cache_write_tokens` (OpenAI) or
  `usage.cache_creation_input_tokens` (Anthropic)
- **THEN** cache write token count is extracted and persisted

#### Scenario: Request model takes precedence

- **WHEN** both the request body and the response indicate a model name
- **THEN** the request model is used as the authoritative model for the captured
  row

#### Scenario: Model extracted from nested Copilot API response

- **WHEN** the request body contains `response.model` (Copilot Responses API
  format)
- **THEN** that nested model name is extracted

#### Scenario: Endpoint kind is captured with every row

- **WHEN** a proxied request is persisted
- **THEN** the row includes an `endpoint_kind` of `inference` or `control_plane`
  determined from the request path, model field, and usage
