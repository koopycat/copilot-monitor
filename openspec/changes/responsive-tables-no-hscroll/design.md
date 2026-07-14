## Context

The dashboard uses standard HTML `<table>` elements with `white-space: nowrap`
on all cells and `overflow-x: auto` on the table wrapper. This ensures data
alignment but forces horizontal scrolling on desktop when the combined column
widths exceed the viewport. The Models table is the primary offender with 8
columns plus inline tags.

The body already uses a responsive full-width margin (`max(2rem, 6vw)`) with no
max-width cap, so tables have the full viewport available.

## Goals / Non-Goals

**Goals:**

- Desktop tables (Models, Sessions) fit within viewport without horizontal
  scrollbar at >=1024px
- Model names and project labels truncate gracefully with ellipsis
- Lower-priority columns (Cache %, Token Reduction) hide at the existing 600px
  mobile breakpoint
- Numeric columns remain compact (tabular-nums, no wrapping needed)

**Non-Goals:**

- No table-component library (stick to native `<table>` per expert
  recommendation)
- No column reordering or user-customizable columns
- No changes to the Routes panel table (it's narrow — path, upstream, capture)
- No responsive breakpoints beyond the existing 600px one

## Decisions

### Strategy: selective nowrap removal + text-overflow ellipsis

Remove `white-space: nowrap` from text-heavy columns only (Model, Project). Add
`text-overflow: ellipsis; overflow: hidden; max-width: <something>` to the Model
column's cell container (the `.bar-cell` flex row). Numeric columns keep
`nowrap` because they're compact and benefit from fixed alignment.

**Alternatives considered:** Remove `nowrap` globally. Rejected — numeric
columns with wrapping look messy (e.g., "1,234" split across lines). Selective
removal is a single-line CSS change per column, not a global rewrite.

### Column priority: hide at 600px

At the existing `@media (max-width: 600px)` breakpoint, add column-hiding via
CSS classes. The Models table hides Cache % and Token Reduction (lower-priority
metadata columns). Model, Requests, Tokens, Latency, Cost — the core data
columns — remain visible. Sessions table: 6 columns already fit at 600px with
reduced font/padding; no hiding needed.

**Alternatives considered:** Responsive column reordering (flexbox table trick).
Rejected — complexity disproportionate to the problem; the existing 600px
breakpoint already handles mobile with reduced padding/font, and column hiding
is the simplest clean fix.

### Implementation: CSS-only

All changes are CSS: selective `white-space: normal` on specific cells,
`text-overflow: ellipsis` on the bar-cell, column-hiding classes at 600px. No
Svelte template changes needed beyond possibly adding a utility class to the
cell markup. This aligns with the project's CSS-first approach.

## Risks / Trade-offs

- **[Risk] Ellipsis hides model metadata** — the bar-cell shows model name +
  endpoint tag + fallback tag + not_billed tag. Truncating with ellipsis could
  hide tags. Mitigation: the `title` attribute on the row already shows full
  detail text; tags are secondary and the most important one (model name) comes
  first in the flex order.
- **[Trade-off] Max-width on Model column may be too aggressive at certain
  widths** — Mitigation: use a relative max-width (`max-width: 40%` or `minmax`)
  rather than a fixed pixel value, so the column uses available space
  proportionally.
