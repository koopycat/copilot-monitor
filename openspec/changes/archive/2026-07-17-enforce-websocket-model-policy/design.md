## Context

The HTTP upgrade request carries no JSON model for Copilot Responses traffic.
The current proxy relays client-to-upstream bytes with `io.Copy`, then observes
model information only in upstream response events. That is too late to prevent
a disallowed model request from reaching the upstream.

## Goals / Non-Goals

**Goals:**

- Apply the existing policy matcher before a complete client text message that
  explicitly names a model is sent upstream.
- Preserve normal WebSocket behavior for allowed traffic, including client frame
  masking and fragmented text messages.
- Give the client a standards-aligned policy close and keep an auditable blocked
  request record.

**Non-Goals:**

- Reject the initial HTTP upgrade with 403; the client cannot send the model
  until it receives a successful upgrade.
- Infer a model from prompt content, credentials, or opaque/binary frames.
- Change the project's documented fail-open rule for empty/unknown models.

## Decision

After the upstream handshake succeeds, the proxy buffers only one complete
client text message (up to the existing 1 MiB inspection limit). It decodes the
mask for inspection while retaining all framing attributes. Before writing that
message upstream, it obtains the model with the existing metadata parser and
evaluates the active policy.

For a blocked model, the buffered message is discarded, a `requests` row is
persisted with status 403 and `stream=true`, and the proxy writes a server-side
WebSocket close frame with code 1008 (policy violation) and reason
`model_blocked`. The upstream has accepted only the handshake; it never receives
the blocked message.

For missing models, invalid JSON, or a message that exceeds the bounded
inspection buffer, the proxy forwards the frame sequence unchanged. This is the
same fail-open rule used for an HTTP request without a model. The implementation
records no body text and does not expand persistence.

## Frame Handling

Frame parsing represents FIN, opcode, RSV bits, masking state, mask key, and a
decoded payload. The writer re-applies the original mask when relaying
client-originated frames and preserves all frame attributes. This both enables
inspection and fixes the previous relay behavior that reconstructed upstream
frames as unfragmented frames.

Control frames may interleave a fragmented text message. They are buffered with
that message until its final fragment makes a policy decision possible, so the
allowed frame sequence remains in its original order.
