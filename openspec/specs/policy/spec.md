<!-- markdownlint-disable MD041 -->

## Purpose

Enforce model-based allow/block policies at the proxy level, with fail-open
semantics, persistent blocked-attempt records, and in-dashboard management.

## Requirements

### Requirement: Global model policy

The system SHALL support a global model allow/block policy with three modes:
`allow_all`, `blocklist`, and `allowlist`.

#### Scenario: No policy configured

- **WHEN** no policy row exists in the database
- **THEN** all models are allowed (default: allow_all)

#### Scenario: Blocklist mode

- **WHEN** policy mode is `blocklist` with models `["gpt-3.5-turbo"]`
- **THEN** requests for `gpt-3.5-turbo` are blocked and all other models are
  allowed

#### Scenario: Allowlist mode

- **WHEN** policy mode is `allowlist` with models `["gpt-4o", "claude-*"]`
- **THEN** only models matching those patterns are allowed

#### Scenario: Allow all mode

- **WHEN** policy mode is `allow_all`
- **THEN** all models are allowed regardless of the models list

---

### Requirement: Blocked model response

Blocked models SHALL return HTTP 403 with a JSON error body identifying the
blocked model.

#### Scenario: Model blocked

- **WHEN** a request's model is blocked by policy
- **THEN** the response is 403 with
  `{"error":"model_blocked","model":"<name>","message":"Model is blocked by policy"}`

---

### Requirement: Blocked attempt persistence

Blocked attempts SHALL be persisted to the requests table with status 403 and
zero token counts.

#### Scenario: Blocked request stored

- **WHEN** a request is blocked by policy and the route capture mode is `usage`
  or `metadata`
- **THEN** a row is inserted with status 403, zero tokens, and zero latency

---

### Requirement: Model pattern matching

Model patterns SHALL support `*` suffix for prefix matching (e.g., `gpt-*`
matches `gpt-4o`).

#### Scenario: Prefix wildcard match

- **WHEN** a pattern `gpt-*` is in the policy models list and the request model
  is `gpt-4o-mini`
- **THEN** the model matches the pattern

#### Scenario: Exact match

- **WHEN** a pattern `gpt-4o` is in the policy models list and the request model
  is `gpt-4o`
- **THEN** the model matches the pattern

#### Scenario: No wildcard match

- **WHEN** a pattern `gpt-4o` is in the policy models list and the request model
  is `claude-3-opus`
- **THEN** the model does not match

---

### Requirement: Fail-open policy evaluation

Policy evaluation SHALL fail open: unknown modes, nil policy, empty model, and
store errors SHALL default to allowing the request.

#### Scenario: Unknown policy mode

- **WHEN** the stored policy mode is not one of allow_all, blocklist, or
  allowlist
- **THEN** all requests are allowed

#### Scenario: Empty model in request

- **WHEN** the request body does not contain a model field
- **THEN** the request is allowed regardless of policy

#### Scenario: Store error during policy load

- **WHEN** the store is unreachable when loading the policy
- **THEN** the last successfully cached policy is used; if no cache exists, the
  request is allowed

---

### Requirement: Policy API management

The policy SHALL be readable and updatable through dashboard API endpoints.

#### Scenario: Read policy

- **WHEN** a GET request is made to `/api/policy`
- **THEN** the current policy mode and models list is returned as JSON

#### Scenario: Update policy

- **WHEN** a PUT request with a valid policy body is made to `/api/policy`
- **THEN** the policy is atomically replaced

#### Scenario: Invalid policy update

- **WHEN** a PUT request with an invalid mode or duplicate model patterns is
  made to `/api/policy`
- **THEN** the request is rejected with a 400 error

---

### Requirement: Model discovery

A model discovery endpoint SHALL return all unique model names from captured
request history.

#### Scenario: Distinct models

- **WHEN** a GET request is made to `/api/policy/models`
- **THEN** a JSON array of all distinct non-empty model names from the requests
  table is returned

---

### Requirement: Policy-constrained model availability

A configured global model policy SHALL constrain both models advertised through
OpenAI-compatible discovery and models accepted for forwarding. Client-side
model configuration SHALL NOT expand the set of models permitted by the proxy
policy.

#### Scenario: Client configuration contains a policy-disallowed model

- **WHEN** a client has a locally configured model that is absent from the proxy
  `allowlist`
- **THEN** the proxy omits that model from its `GET /models` response and
  rejects a direct request for it with the existing blocked-model response

#### Scenario: Policy changes after model discovery

- **WHEN** a client has cached a model from an earlier discovery response and
  that model is subsequently disallowed by the active policy
- **THEN** the proxy rejects the client's later request for that model with the
  existing blocked-model response
