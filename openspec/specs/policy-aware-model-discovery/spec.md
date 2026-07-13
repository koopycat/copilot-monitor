<!-- markdownlint-disable MD041 -->

## Purpose

Filter OpenAI-compatible model discovery responses according to the proxy's
active model policy so clients only see models they are permitted to use.

## Requirements

### Requirement: Policy-aware OpenAI model discovery

The proxy SHALL apply the active global model policy to successful
OpenAI-compatible `GET /models` discovery responses before returning them to a
client. The filtered response SHALL retain the upstream response envelope and
metadata while omitting each `data` entry whose string `id` is not allowed by
the policy.

#### Scenario: Allowlist limits discovered models

- **WHEN** the active policy is `allowlist` with `models` set to
  `["gpt-4o", "claude-*"]` and the upstream `GET /models` response contains
  `gpt-4o`, `claude-3-7-sonnet`, and `gpt-4o-mini`
- **THEN** the proxy returns an otherwise equivalent response containing only
  `gpt-4o` and `claude-3-7-sonnet`

#### Scenario: Blocklist removes discovered models

- **WHEN** the active policy is `blocklist` with `models` set to `["gpt-4o"]`
  and the upstream `GET /models` response contains `gpt-4o` and `gpt-4o-mini`
- **THEN** the proxy returns an otherwise equivalent response containing only
  `gpt-4o-mini`

#### Scenario: Allow-all policy preserves discovery response

- **WHEN** the active policy is `allow_all`
- **THEN** the proxy returns the successful upstream `GET /models` response
  without removing model entries

### Requirement: Resilient model discovery filtering

The proxy SHALL forward a discovery response unchanged when it cannot safely
apply policy filtering. It MUST NOT buffer streaming responses in order to
filter model discovery.

#### Scenario: Unreadable model discovery payload

- **WHEN** a successful `GET /models` response has a JSON content type but
  cannot be parsed as an OpenAI-compatible model-list envelope
- **THEN** the proxy forwards the upstream response unchanged

#### Scenario: Policy unavailable during discovery

- **WHEN** the proxy cannot load a policy and has no cached policy available
- **THEN** the proxy forwards the upstream `GET /models` response unchanged

#### Scenario: Non-discovery response remains streamed

- **WHEN** the proxy forwards a response for any endpoint other than
  `GET /models`
- **THEN** the proxy streams the response without buffering it for model-policy
  filtering
