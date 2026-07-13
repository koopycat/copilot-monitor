## Context

The dashboard places the Models and Recent Sessions tables directly after the
usage chart. Those tables can contain many rows and push route configuration,
policy controls, and the footer far below the initial viewport. The dashboard
already uses native disclosure for the Routes panel, and its utility-oriented
design favors a familiar, low-chrome control over a custom accordion component.

## Goals / Non-Goals

**Goals:**

- Let users independently collapse and expand the Models and Recent Sessions
  sections.
- Preserve the current initial visibility of both tables so the change does not
  hide data unexpectedly on first load.
- Use native, keyboard-operable disclosure semantics with a visible focus state.
- Ensure auto-refresh continues to update the table data whether a section is
  open or closed.

**Non-Goals:**

- Persist section state across browser reloads or devices.
- Collapse the Usage chart, metrics, Routes panel, or Policy panel.
- Change dashboard APIs, data loading, sorting, or table columns.
- Add animation that delays access to data or conflicts with reduced-motion
  preferences.

## Decisions

### Use one native disclosure per table

Each table section will use an independent native disclosure control with the
table heading in its summary. This reuses the dashboard's existing Routes-panel
interaction model and gives pointer, keyboard, and screen-reader support without
JavaScript state or a new dependency.

A custom button plus conditional rendering was rejected because it would require
recreating disclosure semantics and could remove table content during refreshes.
A shared accordion that allows only one open section was rejected because Models
and Recent Sessions are complementary views that users may need to compare.

### Preserve expanded initial state

Both table sections will be open when the dashboard first loads. This preserves
the current data-first layout while allowing users to reduce page length as
needed.

Collapsed state will remain in the native disclosure element for the life of the
loaded page but will not be persisted. Persisting it in local storage was
rejected as unnecessary preference complexity for a local monitoring utility.

### Use a compact section summary

The summary will retain the existing section title hierarchy and use the
dashboard's subdued border, focus, hover, and disclosure-marker styling. The
control will provide a clear visual indication of expanded versus collapsed
state without adding cards, color decoration, or disruptive motion.

## Risks / Trade-offs

- [A collapsed table makes fresh data less immediately visible] → Keep both
  sections expanded initially and retain summary labels at all times.
- [Browser-native disclosure styling varies] → Apply minimal summary and marker
  styles while retaining native semantics.
- [Wide tables remain difficult on narrow screens] → Keep the change scoped to
  disclosure and retain existing responsive table behavior.

## Migration Plan

1. Deploy the updated embedded dashboard assets with the existing binary.
2. Existing dashboard users see both sections open on their next page load.
3. No database, API, or configuration migration is required.
4. Roll back by restoring the prior dashboard asset bundle.

## Open Questions

None.
