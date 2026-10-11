## Context

The store, cost estimates, and dashboard share one token convention:
`prompt_tokens` includes cache reads (`cached_input_tokens` is a subset of it)
and excludes cache writes, which are priced separately. OpenAI chat completions
report usage that way. Anthropic does not: `input_tokens` counts only the
uncached input, and `cache_read_input_tokens` and `cache_creation_input_tokens`
are reported beside it.

Anthropic streams report usage twice. `message_start` carries input and cache
counts; the final `message_delta` carries cumulative output tokens and, in the
current API, cumulative input and cache counts that may also be `null`. Older or
third-party Anthropic-compatible upstreams send output tokens only. GitHub
Copilot's `/v1/messages` returns Anthropic-style usage.

## Goals / Non-Goals

**Goals:**

- Persist Anthropic usage in the shared convention so cost estimates and the
  cache-hit ratio are correct without changes outside capture.
- Keep input and cache counts from earlier stream events when a later event
  omits them or reports them as `null`.
- Keep the row for a response the client already received in full.

**Non-Goals:**

- Migrating rows captured before this change.
- Capturing OpenAI Responses API cache reads
  (`input_tokens_details.cached_tokens`).
- Persisting rows for streams the client cancels mid-response.

## Decisions

### Decision 1: Normalize at capture, not in cost or dashboard code

Cache reads are added to prompt_tokens when usage is parsed. Every consumer
(cost estimates, cache-hit ratio, CLI output, export) then reads one convention.
The alternative, branching on provider in each consumer, would need a stored
marker of the usage shape that rows do not have.

### Decision 2: Add cache reads only when the response did not report `prompt_tokens`

The fold applies when the prompt count comes from `input_tokens` and the cache
count from `cache_read_input_tokens`. Keying it on `input_tokens` alone would be
wrong: the OpenAI Responses API also uses `input_tokens`, and there it already
includes cached tokens. A gateway that reports `prompt_tokens` together with
Anthropic cache fields is kept as reported. With every shape normalized, the
derived total is the same everywhere: prompt + completion + cache write.

### Decision 3: Merge raw usage objects across stream events, larger value wins

The stream observer merges each event's usage object into an accumulated object
field by field and parses the result once. Counts in a stream are cumulative, so
the larger value wins; a missing or `null` field keeps the earlier value, and so
does an explicit `0`. Merging raw objects instead of parsed counts keeps an
upstream-reported `total_tokens` intact, where combining parsed counts could not
tell a reported total from a derived one.

### Decision 4: Persist completed responses with a non-cancelable context

After the response has been streamed in full, the store write uses
`context.WithoutCancel(r.Context())`, so a client hanging up at that point no
longer cancels the insert. The WebSocket path already persists with a background
context.

## Risks / Trade-offs

- [A non-cumulative stream that reports a lower count later would keep the
  higher count] → No supported provider does this; Anthropic and OpenAI usage
  counts only grow within a response.
- [Existing Anthropic-style rows keep the old convention, so historical
  cache-hit ratios stay wrong] → Accepted; there is no stored marker of the
  usage shape, and the cost difference is small because Claude Code sends almost
  all input through the cache.

## Migration Plan

No schema or data migration. The change applies to rows captured after the new
binary runs.

## Open Questions

None.
