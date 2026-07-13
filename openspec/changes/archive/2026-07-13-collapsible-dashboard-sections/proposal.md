## Why

The Models and Recent Sessions tables can dominate the dashboard after a period
with substantial activity. They should be dismissible without hiding the
overview metrics or usage chart that users consult first.

## What Changes

- Make the Models and Recent Sessions dashboard sections independently
  collapsible.
- Preserve each table's current data and empty-state behavior when its section
  is expanded.
- Use an accessible, keyboard-operable disclosure control consistent with the
  dashboard's existing quiet utility UI.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `dashboard`: Make the per-model breakdown and recent sessions table
  independently collapsible.

## Impact

- Affects the Svelte dashboard composition, styles, and dashboard end-to-end
  coverage.
- Does not change dashboard APIs, persistence, or proxy behavior.
