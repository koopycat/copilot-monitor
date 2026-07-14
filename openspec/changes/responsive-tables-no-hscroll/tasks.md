## 1. CSS: selective nowrap and truncation

- [x] 1.1 Remove `white-space: nowrap` from the table rule, apply it selectively
      to numeric cells only (`.num` already exists for this purpose; add
      `white-space: nowrap` to `.num`)
- [x] 1.2 Add `.bar-cell` overflow handling:
      `max-width: min(300px, 40%); overflow: hidden; text-overflow: ellipsis; white-space: nowrap`
      so long model names + tags truncate instead of stretching
- [x] 1.3 Ensure the Sessions table Project column similarly truncates: add
      `max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`
      or a shared `.text-cell` class
- [x] 1.4 Move `overflow-x: auto` on `.table-section-content` into
      `@media (max-width: 600px)` — desktop tables should not have a scroll
      container; mobile keeps it

## 2. Column hiding at 600px breakpoint

- [x] 2.1 Add `.col-optional` class to Models table Cache % and Token Reduction
      `<th>` and corresponding `<td>` cells
- [x] 2.2 Add `@media (max-width: 600px) { .col-optional { display: none; } }`
      to the existing mobile breakpoint
- [x] 2.3 Verify Sessions table fits at 600px without column hiding (6 columns,
      already compact)

## 3. Verification

- [x] 3.1 Verify at 1024px, 1280px, 1440px viewport widths — no horizontal
      scrollbar on either table
- [x] 3.2 Verify at 600px — columns hidden correctly, horizontal scroll allowed,
      layout not broken
- [x] 3.3 Verify the bar-cell inline bar still renders (ellipsis does not hide
      the bar itself)
- [x] 3.4 Run `just dashboard-check` and `just build` — no regressions
- [x] 3.5 Smoke-test the running server: page loads, tables render, API works
