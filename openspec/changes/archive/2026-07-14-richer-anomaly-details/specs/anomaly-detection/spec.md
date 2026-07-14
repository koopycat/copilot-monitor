## MODIFIED Requirements

### Requirement: Unrouted path detection

The system SHALL record an anomaly when a request arrives at a path that matches
no configured route, including the full path, method, model, provider prefix,
and a descriptive detail string that uniquely identifies the path, model, and
provider combination.

#### Scenario: Unknown path recorded

- **WHEN** a POST request arrives at `/v1/chat/completions` with model `gpt-5`
  and provider `openai`, and no route matches
- **THEN** an anomaly is recorded with category `unrouted_path`, severity
  `warn`, path `/v1/chat/completions`, method `POST`, model `gpt-5`, and detail
  `"no route matched: POST /v1/chat/completions (model gpt-5, provider openai)"`

#### Scenario: Unknown path without model

- **WHEN** a GET request arrives at `/unknown` with no model in the request body
  and no route matches
- **THEN** an anomaly is recorded with category `unrouted_path`, severity
  `warn`, path `/unknown`, method `GET`, model empty, and detail
  `"no route matched: GET /unknown"`

### Requirement: Auth header missing detection

The system SHALL record an anomaly when a request arrives at a recognized
Copilot CAPI path without an Authorization header. The anomaly record SHALL
include the model, endpoint, and upstream host from the matched route.

#### Scenario: Chat completion without auth

- **WHEN** a request arrives at `/chat/completions` without an `Authorization`
  header and a route matches
- **THEN** an anomaly is recorded with category `auth_missing`, severity
  `error`, the request path, the matched route's endpoint, the model from the
  request body, and the upstream host from the matched route

#### Scenario: Health endpoint without auth

- **WHEN** a request arrives at `/_health` without an `Authorization` header
- **THEN** no anomaly is recorded (health endpoint does not require auth)
