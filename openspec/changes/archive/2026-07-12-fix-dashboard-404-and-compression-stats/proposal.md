## Why

Two bugs found after recent changes:

1. **Dashboard 404 for proxy requests**: `combinedDashProxy` sends all unmatched
   paths to the dashboard SPA. API/proxy requests that don't match a route get a
   confusing 404 from the dashboard instead of a proper 502 from the proxy.

2. **Compression stats include no_change rows**: `no_change` (ratio 1.0) drags
   the average ratio toward 1.0, displaying "-99%" when compression actually
   saved 50%+ on the requests it did compress.

## What Changes

- `combinedDashProxy` only serves the dashboard for browser requests
  (`Accept: text/html`); API requests get proxy 502 for unmatched paths
- Compression aggregates filter to `compression_status = 'applied'` only
- Tests updated for both fixes

## Capabilities

### Modified Capabilities

- `dashboard`: Browser-only routing in combined mode
- `compression`: Aggregate stats filter to applied-only rows

## Impact

- `internal/cli/run.go`: `combinedDashProxy` + `acceptsHTML` helper
- `internal/store/store.go`: Stats query filter
- `internal/store/sessions.go`: Sessions query filter
- `internal/integration/compression_test.go`: Updated expectations + test helper
  SQL
