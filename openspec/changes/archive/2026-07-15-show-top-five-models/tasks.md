## 1. Model Table Progressive Disclosure

- [x] 1.1 Add local row-disclosure state and derive visible rows by applying the
      five-row limit after sorting the complete model set.
- [x] 1.2 Render the count-bearing `Show all N models` and `Show top 5` native
      button only when more than five rows exist, including `aria-expanded`,
      `aria-controls`, and keyboard-visible focus behavior.
- [x] 1.3 Style the row control with the existing compact dashboard button
      vocabulary and verify that table bar scales and model colors remain stable
      in both views.
- [x] 1.4 Update `dashboard/DESIGN.md` to document the Models table's
      progressive-disclosure behavior and control.

## 2. Browser Coverage

- [x] 2.1 Extend dashboard end-to-end coverage for the five-row default, full
      expansion, return to five rows, dynamic labels, accessibility state, and
      keyboard activation.
- [x] 2.2 Add end-to-end coverage that sorting applies to the complete row set
      before limiting and retains the selected row-disclosure state.
- [x] 2.3 Add end-to-end coverage that five or fewer rows render in full without
      a row-disclosure control.
- [x] 2.4 Verify through end-to-end coverage that row disclosure survives
      refresh and Models section toggles, and that shared rows retain their
      inline bar widths and colors between views.

## 3. Validation

- [x] 3.1 Run `just format`, `just dashboard-check`, and `just dashboard-build`.
- [x] 3.2 Run `just e2e` and verify the dashboard interaction at desktop and
      narrow viewports.
- [x] 3.3 Run `just all` to complete the repository-wide validation suite.
