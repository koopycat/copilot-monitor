# Design System

Copilot Monitor dashboard. Utility-grade, dark, data-first.

## Register

**Product.** Dark terminal-inspired dashboard for LLM API usage monitoring.
Single screen, no chrome, no modals, no sidebars.

## Tokens

### Color (OKLCH, dark theme)

| Token              | Value                   | Role                                  |
| ------------------ | ----------------------- | ------------------------------------- |
| `--bg`             | `oklch(0.14 0.012 255)` | Page background -- deep ink-blue      |
| `--surface`        | `oklch(0.18 0.01 255)`  | Card / section backgrounds            |
| `--border`         | `oklch(0.24 0.008 255)` | Borders, dividers                     |
| `--border-hover`   | `oklch(0.35 0.01 255)`  | Border on hover/focus                 |
| `--ink`            | `oklch(0.87 0.005 255)` | Primary text -- ≥4.5:1 on `--bg` (AA) |
| `--muted`          | `oklch(0.68 0.006 255)` | Secondary text, labels                |
| `--faint`          | `oklch(0.62 0.005 255)` | Tertiary text, column headers         |
| `--accent`         | `oklch(0.7 0.17 250)`   | Links, interactive text               |
| `--accent-fill`    | `oklch(0.55 0.19 250)`  | Active/filled accent backgrounds      |
| `--accent-hover`   | `oklch(0.5 0.18 250)`   | Accent hover state                    |
| `--accent-subtle`  | `oklch(0.26 0.06 250)`  | Subtle accent backgrounds (badges)    |
| `--on-accent`      | `oklch(0.98 0.02 250)`  | Text on accent backgrounds            |
| `--success`        | `oklch(0.72 0.2 145)`   | Success / active session              |
| `--success-subtle` | `oklch(0.24 0.05 145)`  | Success backgrounds                   |
| `--warning`        | `oklch(0.72 0.15 85)`   | Warnings, fallback tags               |
| `--danger`         | `oklch(0.62 0.22 22)`   | Errors, danger states                 |

**Color strategy:** Restrained. Single accent (blue) at ≤10% of surface. Tinted
neutrals with chroma toward the ink-blue hue (255). No warm-tint body background
-- deliberately dark, not cream/sand.

### Spacing (0.25rem steps)

| Token   | Value     | Use                                 |
| ------- | --------- | ----------------------------------- |
| `--s1`  | `0.25rem` | Tight gap (badge padding, inline)   |
| `--s2`  | `0.5rem`  | Standard gap (cards, grid, buttons) |
| `--s3`  | `0.75rem` | Section padding, flex gaps          |
| `--s4`  | `1rem`    | Card padding, grid gap              |
| `--s5`  | `1.25rem` |                                     |
| `--s6`  | `1.5rem`  | Section margin, block spacing       |
| `--s8`  | `2rem`    | Page margin, section margin         |
| `--s10` | `2.5rem`  |                                     |
| `--s12` | `3rem`    | Footer margin                       |

### Z-Index Scale (semantic, ascending)

`dropdown(100)` → `sticky(200)` → `modal-backdrop(300)` → `modal(400)` →
`toast(500)` → `tooltip(600)`

### Radius

| Token              | Use                      |
| ------------------ | ------------------------ |
| `--radius-sm: 4px` | Badges, tags, bar-inline |
| `--radius-md: 6px` | Buttons, inputs, toggles |
| `--radius-lg: 8px` | Cards, sections          |

### Timing

| Token           | Value                            | Use                              |
| --------------- | -------------------------------- | -------------------------------- |
| `--fast: 120ms` | Hover transitions, button states | Button, input, toggle feedback   |
| `--slow: 250ms` | Pulse animations, glow effects   | Loading states, error indicators |

Reduced motion: both set to `0ms` via `@media (prefers-reduced-motion: reduce)`.

## Typography

**Font stack:**
`system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`
**Monospace:** `ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace`

| Element         | Size                  | Weight | Notes                                 |
| --------------- | --------------------- | ------ | ------------------------------------- |
| `h1`            | `1.125rem` (18px)     | 600    | Page title                            |
| `h2`            | `0.8125rem` (13px)    | 600    | Section headers, muted                |
| `body`          | `0.8125rem` (13px)    | 400    | Line-height: 1.55                     |
| `.metric-value` | `1.75rem` (28px)      | 600    | Cost/request count                    |
| `.metric-label` | `0.6875rem` (11px)    | --     | Metric labels, faint                  |
| `table`         | `0.75rem` (12px)      | --     | Data tables                           |
| `th`            | `0.65625rem` (10.5px) | 500    | Column headers, letter-spacing 0.02em |
| `.tag`          | `0.59375rem` (9.5px)  | --     | Inline tags, letter-spacing 0.02em    |
| `footer`        | `0.65625rem` (10.5px) | --     | Faint                                 |

**Line length:** Body text line-length is naturally short (table data, labels).
Headings use `text-wrap: balance`.

## Components

### Card Sections

All collapsible sections share the same visual container:
`border: 1px solid var(--border)`, `border-radius: var(--radius-lg)`,
`background: var(--surface)`, `padding: var(--s3) var(--s4)`.

| Class            | Content                      | Notes                                      |
| ---------------- | ---------------------------- | ------------------------------------------ |
| `.table-section` | Models table, Sessions table | `<details>` with summary + state indicator |
| `.anomaly-feed`  | Anomaly list                 | `<details>` with count badge on summary    |
| `.policy-panel`  | Policy form                  | Always expanded (no `<details>`)           |

**Do not** nest cards. **Do not** add decorative borders or box-shadows.

### Buttons

All buttons share: transparent bg, 1px `var(--border)`, `var(--radius-md)`,
cursor pointer.

| Class                    | Use                       | States                                                      |
| ------------------------ | ------------------------- | ----------------------------------------------------------- |
| `.period-btn`            | Period selection          | `.active`: accent-fill fill                                 |
| `.toggle-btn`            | Granularity/metric toggle | `.active`: accent-fill fill                                 |
| `.btn-sm`, `.btn-cancel` | Generic / cancel          | Hover: surface bg                                           |
| `.model-row-toggle`      | Models row disclosure     | `.btn-sm` styling, count label, `aria-expanded`             |
| `.btn-save`              | Primary action            | accent-fill fill, accent-hover on hover                     |
| `.refresh-btn`           | Manual refresh            | Hover: accent border, active: scale(0.92), `.loading`: spin |

### Tags & Chips

| Class         | Use                          | Style                                |
| ------------- | ---------------------------- | ------------------------------------ |
| `.tag`        | Inline endpoint/model labels | Small border, muted text             |
| `.tag.fb`     | Fallback pricing             | Warning border/color                 |
| `.tag.nb`     | Not billed                   | Faint border/color                   |
| `.token-chip` | Policy model chips           | Accent background, border, monospace |

### Badges

| Class             | Use                      | Style                                            |
| ----------------- | ------------------------ | ------------------------------------------------ |
| `.anomaly-count`  | Anomaly count on summary | Accent-subtle pill bg, accent text, 999px radius |
| `.filter-active`  | Active filter indicator  | Same as anomaly-count (shared styles)            |
| `.severity`       | Anomaly severity         | Uppercase, min-width 3.5rem, bordered            |
| `.severity.warn`  | Warning                  | Yellow border/color                              |
| `.severity.error` | Error                    | Red border/color                                 |
| `.severity.info`  | Info                     | Accent border/color                              |

### Tables

Standard `<table>` with collapsed borders. Numeric columns use `.num`
(tabular-nums, nowrap). Model cells use `.bar-cell` (flex with inline bar). Text
cells use `.text-cell` (ellipsis overflow). Sortable columns use `.sort-header`
(appearance: none button).

The Models table renders the first five rows under the active sort by default
when more than five rows exist. A centered `.model-row-toggle` button reveals
all rows and returns to the compact view, with the complete row count in its
collapsed label. Sorting, inline-bar scaling, and model color assignment use the
complete model collection so disclosure does not alter rankings or visual
encoding.

### Loading & Empty States

- **Loading:** Three animated dots (`.loading`, `.loading-dot`) with staggered
  150ms delay. Reduced motion: static 60% opacity.
- **Empty state:** Centered faint text (`.empty-state`). Differs from `.empty`
  (used inside sections).
- **Error state:** Centered danger text (`.error-state`).
- **No activity state:** `.periodIsEmpty` shows "No activity captured for this
  period" message.

## Brand Rules

- **No** side-stripe borders (>1px colored border-left/right)
- **No** gradient text
- **No** glassmorphism
- **No** hero-metric templates
- **No** identical card grids
- **No** border-radius > 16px on any card
- **Dark theme only** -- no light theme toggle (matches developer aesthetic)
- **No** warm-tinted backgrounds (cream/sand/paper)
- **No** 1px border + wide box-shadow on same element (ghost-card pattern)

## Responsive

Single breakpoint at 600px:

- Body margin shrinks
- Metric grid uses `minmax(140px, 1fr)`
- Metric value drops to 1.375rem
- `.col-optional` hidden
- `.table-section-content` gains `overflow-x: auto`
