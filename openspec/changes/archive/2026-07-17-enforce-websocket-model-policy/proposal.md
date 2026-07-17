## Why

The model policy applies to HTTP requests before forwarding, but Copilot
Responses WebSocket traffic currently upgrades before its model is visible. The
connection can therefore send a model-bearing request without the proxy applying
the configured allowlist or blocklist.

## What Changes

- Inspect complete client-to-upstream WebSocket text messages before forwarding
  them.
- When a message explicitly names a policy-disallowed model, do not forward the
  model-bearing message, persist a status-403 blocked attempt, and close the
  WebSocket with the standard policy-violation code and `model_blocked` reason.
- Preserve masking, fragmentation, FIN, opcode, and reserved bits while relaying
  allowed frames in either direction.
- Document the precise fail-open boundary for messages with no usable model,
  invalid JSON, or over-limit inspection payloads.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `policy`: Enforce configured model policy for explicitly model-bearing
  WebSocket messages.
- `proxy`: Safely inspect and relay bidirectional WebSocket frames.

## Impact

- Changes local proxy behavior only; no schema migration or dashboard API is
  required.
- Adds isolated frame-relay tests without a real external WebSocket upstream.
- Replaces the documented blanket WebSocket policy bypass with a bounded,
  explicit fail-open rule that matches existing empty-model semantics.
