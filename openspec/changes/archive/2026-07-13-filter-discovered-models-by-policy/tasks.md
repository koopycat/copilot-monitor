## 1. Policy and discovery helpers

- [x] 1.1 Extract cached policy loading into a handler helper usable by
      discovery filtering and request-time enforcement.
- [x] 1.2 Add OpenAI-compatible model-list parsing and filtering that preserves
      the response envelope and removes entries disallowed by the active policy.
- [x] 1.3 Add a bounded response path for successful JSON `GET /models` requests
      while retaining streaming forwarding for every other response.

## 2. Proxy enforcement integration

- [x] 2.1 Apply policy-aware filtering before committing the upstream
      `GET /models` response headers and retain the existing request-time block.
- [x] 2.2 Preserve unchanged discovery forwarding for `allow_all`, unavailable
      policy, malformed discovery JSON, and unsupported discovery envelopes.

## 3. Verification and documentation

- [x] 3.1 Add proxy tests for allowlist and blocklist model discovery, wildcard
      matching, envelope preservation, and direct-request enforcement after
      discovery.
- [x] 3.2 Add resilience tests proving malformed discovery payloads fail open
      and non-discovery streaming responses are not buffered.
- [x] 3.3 Update the policy architecture documentation to describe
      policy-constrained model discovery.
- [x] 3.4 Run `just test` and `just all`.
