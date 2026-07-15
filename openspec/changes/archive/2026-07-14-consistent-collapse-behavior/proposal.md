## Why

The dashboard has four collapsible sections but uses two different collapse
indicators. Models and Sessions show a custom "collapsed"/"expanded" text label
alongside the native `<details>` arrow. Anomalies and Routes use only the native
arrow. This inconsistency breaks visual rhythm and adds UI noise (the text
labels are redundant with the arrow).

## What Changes

Remove the custom `.table-section-state` element and its CSS rules from Models
and Sessions sections. All collapsible sections will use the native `<details>`
disclosure arrow as their sole collapse indicator.

## Capabilities

### Modified Capabilities

- `dashboard-design-system`: The `.table-section` collapse indicator SHALL use
  only the native `<details>` disclosure arrow, matching the behavior of
  `.anomaly-feed` and `.routes-panel`.

## Impact

- **Markup**: `dashboard/src/App.svelte` -- remove two
  `<span class="table-section-state">` elements
- **CSS**: `dashboard/src/app.css` -- remove `.table-section-state::after` and
  `.table-section[open] .table-section-state::after` rules
- **No breaking changes**: Visual only, all `<details>` functionality preserved
