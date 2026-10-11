## Why

Issue #68 reported that Anthropic-style usage is under-counted. Reproducing it
confirmed two capture bugs and found a third:

- Anthropic `input_tokens` excludes cache reads, but cost estimates and the
  dashboard treat `prompt_tokens` as including them (cost prices
  `prompt_tokens - cached_input_tokens` as uncached input; cache-hit % is
  `cached_input_tokens / prompt_tokens`). This affects GitHub Copilot's own
  `/v1/messages` traffic, not only Anthropic-direct upstreams: rows such as
  prompt 9 with 20,108 cache reads under-price input and show cache-hit
  percentages far above 100%.
- A streamed response keeps only the last usage object. When the final Anthropic
  `message_delta` carries only output tokens, or carries input and cache counts
  as `null`, the input and cache counts from `message_start` are lost.
- When a client disconnects right after reading a complete response, the row is
  not persisted because the store write is canceled with the client request.

## What Changes

- Normalize Anthropic usage at capture: prompt_tokens is input tokens plus cache
  read tokens, matching the OpenAI convention the store, cost estimates, and
  dashboard already assume. A response that reports `prompt_tokens` itself is
  kept as reported.
- Combine usage reported across the events of a streamed response instead of
  keeping only the last event; each count keeps the largest value reported.
- Persist the row for a completed response even if the client has already
  disconnected.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `capture`: Usage metadata capture defines how Anthropic input tokens and cache
  reads map to prompt_tokens, that reported prompt_tokens are kept as reported,
  and how usage from multiple stream events is combined.

## Impact

- **Proxy (`internal/proxy`)**: usage parsing and the stream observer; the
  persistence context after a completed response.
- **Cost and dashboard**: no code change; Anthropic rows captured after this
  change price uncached input correctly and show cache-hit percentages of at
  most 100%.
- **Existing rows**: not migrated. Rows captured before this change keep the old
  convention (prompt_tokens without cache reads for Anthropic-style usage).
- **Out of scope**: OpenAI Responses API cache reads
  (`input_tokens_details.cached_tokens`) are not captured yet, and rows for
  streams the client cancels mid-response are still not persisted.
