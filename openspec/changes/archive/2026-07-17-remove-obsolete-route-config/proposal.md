## Why

The proxy was simplified to one required `--upstream` host, but the repository
still contains retired route JSON examples, an unused dashboard Routes panel,
and a placeholder `/api/config` endpoint. Those artifacts imply multi-provider
routing that no longer exists and conflict with the product's local-monitor
positioning.

## What Changes

- Remove obsolete route JSON files and route-based issue-template guidance.
- Remove the unused dashboard Routes panel, state, API client, styles, and
  placeholder `/api/config` endpoint.
- Remove the fixed `capture_mode` log field and update test names, specs, and
  design docs to describe the single-upstream proxy only.
- Correct stale client base-URL documentation that still adds a retired path
  prefix.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `reporting`: Remove the retired configuration API from the dashboard surface.
- `dashboard`: Remove the retired route-configuration display.
- `capture`: Describe capture without retired per-route modes.

## Impact

- Removes an undocumented endpoint that only returns a placeholder and is not
  useful to the current dashboard.
- Does not affect proxy forwarding, persistence, or current documented API
  endpoints.
- Deletes stale example files rather than preserving invalid configuration as a
  misleading compatibility promise.
