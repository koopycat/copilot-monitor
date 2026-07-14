## Context

The Models table renders a "Token Reduction" column showing estimated prompt tokens saved by proxy-side Headroom compression. The user has determined this metric belongs in Headroom's own tooling, not in copilot-monitor's dashboard. The column shows "–" for most rows (no compression configured) and displays a misleading percentage.

## Goals / Non-Goals

**Goals:**
- Remove the "Token Reduction" column from the Models table
- Clean up the `col-optional` class from the removed `<th>` and `<td>`

**Non-Goals:**
- No API changes — `/api/stats` still returns compression fields for programmatic consumers and CSV export
- No schema changes — QUAL-006 forbids data loss
- No change to the `ModelStats` TypeScript type — frontend types stay in sync with the API response

## Decisions

### Remove display only, keep data pipeline

Delete the `<th>` and `<td>` from `ModelsTable.svelte`. The `compression_*` fields in `ModelStats` and the API remain untouched — they're still part of the `/api/stats` response and CSV export, available for anyone who wants them programmatically or via `headroom perf`.

## Risks / Trade-offs

None. Pure display removal. No behavioral change.
