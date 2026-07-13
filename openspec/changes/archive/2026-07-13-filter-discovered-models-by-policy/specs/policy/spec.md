## ADDED Requirements

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
