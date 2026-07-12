## Context

Compression statistics aggregate across all requests with
`compression_status IN ('applied', 'no_change')`. `no_change` requests have
`original_tokens == final_tokens`, so their ratio is 1.0. When most requests are
`no_change` (e.g., under 500 tokens, Headroom's minimum), the average ratio is
near 1.0 — displaying as "-99%" or "-100%".

## Goals / Non-Goals

**Goals:**

- Only count `applied` rows in compression aggregates

**Non-Goals:**

- Change status labels or persistence
- Change dashboard or API schema

## Decisions

### Filter to `applied` only, not `applied` + `no_change`

**Why**: `no_change` means zero impact. Including it in averages makes the
numbers useless. Users want to see "compression saved X tokens across Y
requests" — not "across Y+Z requests where Z did nothing."

No alternative considered — the zero-impact rows have no informational value in
an aggregate.
