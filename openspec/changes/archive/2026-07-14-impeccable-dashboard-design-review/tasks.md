## 1. Impeccable Critique

- [x] 1.1 Run `$impeccable critique` on the dashboard to get UX heuristic scores
      and identify P0/P1 issues
- [x] 1.2 Document critique findings as the backlog of design issues to address

## 2. Impeccable Audit

- [x] 2.1 Run `$impeccable audit` on the dashboard for technical quality checks
      (a11y, contrast, perf, responsive)
- [x] 2.2 Document audit findings with specific file/line references

## 3. Color Palette Migration

- [x] 3.1 Define new OKLCH-based CSS custom properties replacing the GitHub-dark
      hex palette
- [x] 3.2 Verify all text/background contrast ratios meet WCAG AA (4.5:1 body,
      3:1 large)
- [x] 3.3 Ensure placeholder and muted text also meet 4.5:1 contrast threshold

## 4. Typography Refinement

- [x] 4.1 Define explicit type scale with clamp() for responsive sizing
- [x] 4.2 Cap body text line length at 68ch
- [x] 4.3 Add `text-wrap: balance` on headings
- [x] 4.4 Ensure heading letter-spacing is no tighter than -0.04em

## 5. Layout and Spacing

- [x] 5.1 Define consistent spacing scale (0.25rem increments) for vertical
      rhythm
- [x] 5.2 Convert metric card row to `repeat(auto-fit, minmax(...))` grid
- [x] 5.3 Implement semantic z-index scale (dropdown → sticky → modal-backdrop →
      modal → toast → tooltip)
- [x] 5.4 Remove `scrollbar-gutter: stable` or assign a real purpose

## 6. Motion and Interactions

- [x] 6.1 Add purposeful reveal animations for chart and table sections
- [x] 6.2 Add hover micro-interactions on interactive cards
- [x] 6.3 Ensure all animations respect `prefers-reduced-motion: reduce`
- [x] 6.4 Refine session pulse glow timing curve

## 7. Anti-Pattern Removal

- [x] 7.1 Remove ghost-card patterns (border + box-shadow combos) from cards and
      buttons
- [x] 7.2 Cap border-radius at 16px for cards/sections
- [x] 7.3 Remove any side-stripe border accents, gradient text, or decorative
      backgrounds
- [x] 7.4 Audit for and remove any section eyebrow/kicker patterns

## 8. Loading and Empty States

- [x] 8.1 Add loading indicator during initial data fetch
- [x] 8.2 Improve empty-state messaging for no-data periods
- [x] 8.3 Add empty-state for upstream filter with no matches

## 9. Impeccable Polish Pass

- [x] 9.1 Run `$impeccable polish` for final quality pass on the refined
      dashboard
- [x] 9.2 Address any remaining P1/P2 issues from polish output

## 10. Validation

- [x] 10.1 Test dashboard visually at 400px, 800px, and full width
- [x] 10.2 Test with `prefers-reduced-motion: reduce` enabled
- [x] 10.3 Verify no visual regressions in dark mode rendering
- [x] 10.4 Run `just build` to confirm embedded assets compile successfully
