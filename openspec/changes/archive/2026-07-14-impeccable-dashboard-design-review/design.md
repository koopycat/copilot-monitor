## Context

The dashboard is a Svelte 5 single-page app embedded via `go:embed` into the
copilot-monitor binary. It uses global CSS (`app.css`) for all styling with CSS
custom properties for tokens. No runtime CSS framework, no build-time CSS
pipeline beyond Vite's default CSS bundling. The design follows a utilitarian
dark theme based on GitHub's primer palette. All interactive elements work but
lack visual refinement.

## Goals / Non-Goals

**Goals:**

- Apply the impeccable design methodology (critique → audit → polish) to the
  dashboard
- Establish a deliberate color strategy with OKLCH and verified contrast ratios
- Refine typography hierarchy, spacing rhythm, and layout proportions
- Add purposeful motion and micro-interactions
- Eliminate visual anti-patterns per impeccable's absolute bans
- Maintain all existing functionality and API contracts unchanged

**Non-Goals:**

- No new npm dependencies or CSS frameworks (per QUAL-003: embedded static
  assets)
- No functional/behavioral changes to dashboard capabilities
- No server-side Go changes
- No design-system extraction or token library (defer to future
  `impeccable extract`)
- No responsive redesign for mobile-first (defer to future `impeccable adapt`)

## Decisions

### Methodology: Impeccable critique → audit → polish pipeline

Run the dashboard through impeccable's evaluate and refine commands in sequence:
`critique` for UX heuristic scoring, `audit` for technical quality (a11y, perf,
responsive), then `polish` for the final quality pass. This mirrors the skill's
prescribed workflow for existing surfaces that have never been reviewed.

**Alternatives considered:** Running `bolder` directly for a quick visual boost.
Rejected because the dashboard needs a systematic review first -- boldness
without fixing structural issues amplifies defects.

### Color Strategy: Committed dark with accent restraint

Current palette is a direct GitHub-dark clone (#0d1117 bg, #58a6ff accent). This
works but reads as "GitHub tool" rather than having its own identity. The new
palette will:

- Keep dark mode (the dashboard is a monitoring tool used alongside
  IDEs/terminals)
- Shift the background slightly off GitHub-dark with OKLCH: deeper black-blue
  with 0.005 chroma toward accent hue
- Use a "restrained" color strategy: tinted neutrals + one accent at ≤10%
  surface usage
- Maintain the existing green/red/yellow semantic colors for status
  (success/danger/warning)
- Verify all contrast ratios against WCAG AA

**Alternatives considered:** Light mode. Rejected -- the scene is "developer
monitoring API usage, in a terminal/IDE context, focused analytical work." Dark
mode matches the ambient context.

### Typography: Keep system-ui, refine scale

System font stack is correct for a developer tool (no web font latency, matches
OS). Changes limited to:

- Define explicit type scale with clamp() for responsiveness
- Cap body line length at 68ch
- Increase heading weight differentiation
- Use `text-wrap: balance` on h1/h2
- Monospace for code/data values only (already correct)

**Alternatives considered:** Adding Inter or JetBrains Mono via Google Fonts.
Rejected -- adds network dependency and latency; system fonts are appropriate
for an embedded monitoring tool.

### Layout: Grid refinement without breakpoints

Current layout uses flexbox with `flex-wrap` (correct choice for metric cards).
Improvements:

- Define a consistent spacing scale (0.25rem increments)
- Use `repeat(auto-fit, minmax(...))` for the metric card row
- Add vertical rhythm between sections
- Fix the `scrollbar-gutter: stable` value (currently unused/incorrect)

**Alternatives considered:** CSS Grid for the main layout. Rejected -- flex-wrap
is simpler for the 1D card row, which is the main layout concern.

### Motion: Purposeful micro-interactions

Current motion: toggle transitions, session pulse glow, view transitions.
Improvements:

- Stagger chart and table entry animations
- Add hover scale on interactive cards
- Refine the session pulse timing curve
- Ensure all animations respect `prefers-reduced-motion`

**Alternatives considered:** Adding a motion library (motion, GSAP). Rejected --
no new dependencies; CSS-only animations are sufficient for this scope.

### Implementation: CSS-only, no component restructure

All changes live in `dashboard/src/app.css` plus targeted edits to `App.svelte`
for markup adjustments. No component file refactoring, no new files, no build
pipeline changes. The embedded build constraint (QUAL-003) makes this the only
viable approach.

## Risks / Trade-offs

- **[Risk] Breaking visual layout on narrow viewports** → Mitigation: test at
  400px and 800px widths; keep existing `@media (max-width: 600px)` breakpoint
- **[Risk] Contrast regression with OKLCH migration** → Mitigation: verify every
  new color pair with a contrast checker before committing
- **[Trade-off] No design tokens file** → By not extracting to a formal token
  system, future styling changes will be manual. Acceptable for this scope;
  `impeccable extract` can follow later
- **[Trade-off] Design review is subjective** → The impeccable skill provides
  objective criteria (contrast ratios, heuristic scores, anti-pattern bans). The
  polish pass is opinionated but follows the skill's established design guidance
