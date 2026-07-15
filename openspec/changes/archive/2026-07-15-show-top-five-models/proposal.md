## Why

Installations that use many models make the Models table dominate the dashboard
and push lower sections out of view. Showing only the most relevant five rows
initially keeps the single-screen dashboard compact while preserving access to
the complete breakdown.

## What Changes

- Limit the Models table to its first five sorted rows by default.
- When more than five models are available, provide an inline control to show
  all models and return to the top-five view.
- Apply sorting to the complete model set before selecting the five visible
  rows, so the default view shows the five models with the most tokens and
  subsequent sorts show the leading five for the active sort.
- Keep row expansion independent from the Models section's existing expanded or
  collapsed state.
- Keep the complete model data available in the browser without changing API
  responses or exports.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `dashboard`: Change the per-model breakdown to use top-five progressive
  disclosure while retaining sorting and access to every model.

## Impact

- Affects the dashboard Models table behavior, its inline control styling, and
  dashboard UI coverage.
- Does not change APIs, persistence, CLI behavior, embedded-build architecture,
  or runtime dependencies.
