## Why

The dashboard is functional but visually generic -- a stock GitHub-dark palette,
system-ui defaults, and utilitarian spacing that reads as "engineer-built
monitoring tool" rather than a polished product interface. A design review using
the impeccable methodology will elevate visual hierarchy, typography, color
strategy, layout rhythm, and interactive polish to a production-grade standard
without changing any functional behavior.

## What Changes

- Design audit of the dashboard frontend (critique + audit phases from
  impeccable)
- Refined color palette using OKLCH with intentional contrast and tint strategy
- Improved typography: better hierarchy, line-length control, font pairing
- Layout improvements: spacing rhythm, semantic z-index scale, responsive
  refinements
- Motion and micro-interactions: reveals, hover transitions, reduced-motion
  support
- Polish pass on UX copy, empty states, and loading states
- Removal of visual anti-patterns (overly rounded corners, border+shadow
  ghost-cards, etc.)
- Accessibility: contrast verification, focus indicators, keyboard navigation
  polish

## Capabilities

### New Capabilities

- `dashboard-design-system`: Refined visual design system for the dashboard
  including color tokens, typography scale, spacing rhythm, motion primitives,
  and component-level styling that applies to all existing dashboard UI elements

### Modified Capabilities

None. This change is purely visual/aesthetic -- no functional requirements, API
endpoints, or user-facing behaviors are added, modified, or removed. The
existing `dashboard` spec requirements remain unchanged.

## Impact

- Affected code: `dashboard/src/app.css` (global styles),
  `dashboard/src/App.svelte` (layout markup), all
  `dashboard/src/components/*.svelte` (component-level styling)
- No API changes, no backend Go code changes, no schema changes
- Embedded static assets (`dashboard/dist/`) regenerated on rebuild
- No breaking changes
