## Why

The dashboard's Models and Sessions tables require horizontal scrolling on
common desktop displays (1024-1440px) because `white-space: nowrap` forces every
column to its maximum width and `overflow-x: auto` hides the excess behind a
scrollbar. This obscures data and violates PRODUCT.md's data-first principle —
users should see all columns without panning.

## What Changes

- Tables fit within the desktop viewport without horizontal scrolling
  at >=1024px
- Model names and project labels truncate with ellipsis when they exceed
  available column width rather than stretching the table
- Numeric columns keep their compact layout; only text-heavy columns (Model,
  Project) adapt
- Lower-priority columns hide at the existing 600px mobile breakpoint
- The `overflow-x: auto` on table sections becomes mobile-only, not active on
  desktop

## Capabilities

### New Capabilities

None. This is a refinement within the existing dashboard capability.

### Modified Capabilities

- `dashboard`: ADDED requirement for desktop table fit without horizontal
  scrolling

## Impact

- Affected files: `dashboard/src/app.css` (table/media-query rules),
  `dashboard/src/components/ModelsTable.svelte` (column markup), possibly
  `dashboard/src/components/SessionsTable.svelte`
- No API changes, no backend changes, no new dependencies
