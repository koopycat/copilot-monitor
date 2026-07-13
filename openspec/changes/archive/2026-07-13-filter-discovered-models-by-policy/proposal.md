## Why

A proxy allowlist currently rejects disallowed requests only after a client has
discovered and selected a model. This leaves tools presenting models that will
inevitably fail, and makes a client-side allowlist appear to override the
configured security policy.

## What Changes

- Filter OpenAI-compatible upstream model-discovery responses according to the
  active proxy model policy.
- Treat a proxy allowlist as the authoritative available-model set for clients
  using the proxy.
- Preserve upstream discovery behaviour when the policy is `allow_all` or when
  policy evaluation must fail open.
- Keep request-time policy enforcement as the final safeguard against stale
  discovery results or clients that submit model IDs directly.

## Capabilities

### New Capabilities

- `policy-aware-model-discovery`: Expose only policy-permitted models to clients
  that discover models through the proxy.

### Modified Capabilities

- `policy`: Make a configured model policy constrain both model discovery and
  request forwarding.

## Impact

- Affected proxy handling of OpenAI-compatible `/models` responses and policy
  evaluation reuse.
- Affected tests for response forwarding and policy enforcement.
- No new dependencies, configuration, or database schema changes.
