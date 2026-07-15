<!-- markdownlint-disable MD041 -->

## Purpose

Define visual design system requirements for the dashboard including color
palette, typography, layout, motion, and anti-pattern avoidance.

## Requirements

### Requirement: Color palette uses OKLCH with verified contrast

The dashboard SHALL use an OKLCH-based color palette where body text meets WCAG
AA contrast (≥4.5:1) against the page background, large text meets ≥3:1, and
placeholder text meets the same 4.5:1 threshold as body text.

#### Scenario: Body text contrast

- **WHEN** the dashboard renders any body text on the page background
- **THEN** the contrast ratio is at least 4.5:1

#### Scenario: Large metric text contrast

- **WHEN** the dashboard renders metric values (≥18px bold)
- **THEN** the contrast ratio is at least 3:1

#### Scenario: Placeholder text contrast

- **WHEN** the dashboard renders placeholder text (e.g., in input fields, empty
  states)
- **THEN** the contrast ratio is at least 4.5:1

---

### Requirement: Typography scale with line-length constraints

The dashboard SHALL use a defined typography scale where body text line length
is capped at 65-75ch, display headings use clamp() with a max of 6rem, and
display heading letter-spacing is no tighter than -0.04em.

#### Scenario: Body text line length

- **WHEN** the dashboard renders body text
- **THEN** the line length does not exceed 75 characters

#### Scenario: Heading sizing

- **WHEN** the dashboard renders headings
- **THEN** no heading exceeds 6rem in computed font size

#### Scenario: Heading letter-spacing floor

- **WHEN** the dashboard renders display headings
- **THEN** letter-spacing is no tighter than -0.04em

---

### Requirement: Semantic z-index scale

The dashboard SHALL use a semantic z-index scale with ordered layers (dropdown,
sticky, modal-backdrop, modal, toast, tooltip) rather than arbitrary numeric
values.

#### Scenario: Overlapping elements

- **WHEN** overlapping UI elements render (autocomplete listbox, sticky headers,
  modals)
- **THEN** their z-index values follow an ascending semantic order from lowest
  to highest priority

---

### Requirement: Reduced motion support

The dashboard SHALL respect the user's `prefers-reduced-motion: reduce`
preference by replacing animations with instant transitions or crossfades.

#### Scenario: Reduced motion preference active

- **WHEN** the user has `prefers-reduced-motion: reduce` enabled
- **THEN** all CSS animations and transitions use zero or near-zero duration

#### Scenario: Reduced motion preference absent

- **WHEN** the user does not have reduced motion enabled
- **THEN** animations and transitions use their designed durations

---

### Requirement: No visual anti-patterns

The dashboard SHALL not use side-stripe borders (>1px colored border-left/right
on cards), gradient text (background-clip: text), glassmorphism decorations,
hero-metric templates, or identical icon-card grids.

#### Scenario: Card borders

- **WHEN** the dashboard renders cards, callouts, or list items
- **THEN** no element uses a single colored side-border accent (>1px on one
  side)

#### Scenario: Text color

- **WHEN** the dashboard renders any text
- **THEN** no text uses background-clip: text with a gradient

#### Scenario: Card border-radius

- **WHEN** the dashboard renders cards or sections
- **THEN** no card has a border-radius exceeding 16px

---

### Requirement: Loading and empty states

The dashboard SHALL display meaningful loading states during data fetches and
clear empty-state messages when no data exists for the selected period or
filter.

#### Scenario: Initial load

- **WHEN** the dashboard first loads and data is being fetched
- **THEN** a loading indicator is visible

#### Scenario: Empty period

- **WHEN** no data exists for the selected period
- **THEN** a clear empty-state message is displayed

#### Scenario: Empty after filter

- **WHEN** an upstream filter is applied and no data matches
- **THEN** a clear message indicates the filter has no matches

---

### Requirement: Collapsible section visual consistency

All collapsible dashboard sections SHALL use a consistent bordered card
container style: a 1px border in `var(--border)`, border radius of
`var(--radius-lg)`, surface background (`var(--surface)`), and internal padding.
All collapsible sections SHALL use the native `<details>` disclosure arrow as
the sole collapse indicator. No section SHALL display custom
"collapsed"/"expanded" text labels.

#### Scenario: Models section has card style

- **WHEN** the dashboard loads and the Models section is visible
- **THEN** the section appears as a bordered card matching the Anomalies section
  style, with only the native disclosure arrow as its collapse indicator

#### Scenario: Sessions section has card style

- **WHEN** the dashboard loads and the Recent Sessions section is visible
- **THEN** the section appears as a bordered card matching the Anomalies section
  style, with only the native disclosure arrow as its collapse indicator

#### Scenario: All sections use the same indicator

- **WHEN** the dashboard is fully loaded
- **THEN** all collapsible sections (Models, Sessions, Anomalies, Routes) show
  identical collapse indicators

#### Scenario: Sections remain independently collapsible

- **WHEN** the user toggles any collapsible section
- **THEN** only that section expands or collapses while the card border and
  header remain visible
