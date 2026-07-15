## Context

The Models table currently sorts the complete client-side model breakdown and
renders every row. That preserves detail but allows the section to grow without
bound, which conflicts with the dashboard's single-screen, data-first design.
The Models section already has its own `<details>` disclosure, so row-level
progressive disclosure must remain visually and behaviorally distinct from
collapsing the whole section.

## Goals / Non-Goals

**Goals:**

- Render at most five model rows when the table first appears.
- Make the visible five follow the table's active sort, including the existing
  total-token descending default.
- Let mouse, keyboard, and assistive-technology users reveal every row and
  return to the compact view.
- Preserve the existing table columns, sorting behavior, bar scaling, model
  colors, and full API data.
- Keep the control quiet and consistent with the dashboard's existing utility
  styling.

**Non-Goals:**

- Changing model aggregation, API response ordering, database queries, CSV
  exports, or chart grouping.
- Loading additional model data on demand or reducing network payload size.
- Persisting the row disclosure choice across page reloads.
- Replacing or changing the Models section's existing disclosure control.

## Decisions

### Limit the fully sorted client-side rows

The table will continue to sort the complete model-row collection and will
derive its rendered rows by taking the first five unless the user has selected
the all-rows view. The default therefore remains the five highest-token rows,
while choosing another sort column makes the compact view show the leading five
rows for that sort. Bar scaling and model color assignment will continue to use
the complete collection so expanding or collapsing rows does not change the
visual encoding.

A fixed token-based top five was rejected because it would make other sort
headers appear ineffective for rows outside that fixed subset. A server-side
limit was rejected because it would prevent complete client-side sorting,
require another request to expand, and add API complexity without addressing a
data-volume problem.

### Keep row disclosure as local component state

A local boolean will default to the compact view and derive the visible rows
without mutating the store data. The choice will remain stable for the
component's lifetime, including sorting, auto-refresh, and collapsing or
reopening the Models section. When five or fewer rows are available, all rows
will render and the disclosure control will be omitted regardless of the local
state.

A nested `<details>` element was rejected because the Models section already
uses that affordance to hide the whole table and a second disclosure marker
would make the two scopes ambiguous.

### Use one explicit native button below the table

When more than five rows exist, a native button immediately below the table will
read `Show all N models` in compact mode and `Show top 5` in expanded mode. It
will expose its state with `aria-expanded` and identify the controlled table
body with `aria-controls`. The native button provides keyboard activation and
focus behavior without custom event handling. The control will reuse the
existing small-button visual vocabulary and will not animate the row transition.

The exact model count in the compact label makes the amount of hidden data clear
and avoids a vague `More` affordance. Placing the control below the table keeps
it discoverable at the boundary where rows stop without competing with sortable
column headers.

### Cover the behavior through browser end-to-end tests

The existing deterministic dashboard seed contains more than five model rows, so
browser coverage can verify the initial five-row limit, both button states,
complete expansion, return to five rows, keyboard operation, and sorting before
limiting. Dashboard type checking and the production dashboard build will remain
the static validation gates.

## Risks / Trade-offs

- [Hidden rows are not searchable in the rendered page until expanded] -> The
  explicit count-bearing control makes the hidden rows discoverable and exposes
  all rows in one action.
- [Preserving the expanded choice across a period or filter change can produce a
  tall table again] -> This respects deliberate user intent for the current page
  session, while a reload restores the compact default.
- [The change does not reduce API or browser memory use] -> The goal is visual
  density, and retaining all rows client-side keeps sorting and expansion
  immediate.

## Migration Plan

Ship the updated pre-built dashboard assets with the existing binary build
process. No database, API, or configuration migration is required. Rollback
consists of restoring the previous table rendering and removing the row
disclosure styling and tests.

## Open Questions

None.
