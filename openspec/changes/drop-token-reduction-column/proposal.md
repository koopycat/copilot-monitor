## Why

The Models table displays a "Token Reduction" column showing estimated prompt tokens saved by proxy-side Headroom compression. This metric belongs to Headroom's own tooling (`headroom perf`), not to copilot-monitor. It currently displays a misleading percentage (shows remaining ratio as a negative, not actual reduction) and shows "–" for the vast majority of rows where no compression route is configured. The column adds visual noise to the primary monitoring view without actionable value in this context.

## What Changes

- Remove the "Token Reduction" column from the Models table (both `<th>` and `<td>`)
- No API, schema, or data changes — compression metrics remain available in `/api/stats` and CSV export for programmatic consumers
- The `col-optional` class added for this column in the prior responsive-tables change is removed from it (Cache % still uses it)

## Capabilities

### Modified Capabilities

- `dashboard`: MODIFIED requirement for per-model breakdown — the table no longer includes a compression/token-reduction column

## Impact

- Affected files: `dashboard/src/components/ModelsTable.svelte` only
- No API changes, no schema changes, no backend changes
- CSV export still carries compression fields for users who need them
