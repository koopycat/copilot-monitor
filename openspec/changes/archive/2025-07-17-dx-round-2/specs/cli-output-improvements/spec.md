## ADDED Requirements

### Requirement: today prints friendly date header

The `today` command SHALL print a human-readable date header instead of RFC3339
format.

The header SHALL use the format `"Usage for <Month> <Day>, <Year>"` (e.g.,
"Usage for July 17, 2025").

#### Scenario: today output

- **WHEN** `copilot-monitor today` is executed on July 17, 2025
- **THEN** stdout begins with `"Usage for July 17, 2025\n"`

### Requirement: rebuild-sessions reports counts and timing

The `rebuild-sessions` command SHALL print the number of sessions rebuilt, the
number of requests processed, and the elapsed time upon successful completion.

The output SHALL use the format
`"rebuilt <N> sessions from <M> requests in <duration>"`.

#### Scenario: Successful rebuild

- **WHEN** `copilot-monitor rebuild-sessions` completes successfully
- **THEN** stdout SHALL contain the rebuilt session count, request count, and
  elapsed duration

#### Scenario: rebuild with vacuum

- **WHEN** `copilot-monitor rebuild-sessions --vacuum` completes successfully
- **THEN** stdout SHALL still report counts and timing before the vacuum message

### Requirement: cost shows compression removed tokens

The `cost` command SHALL display a `TOKENS_REMOVED` column showing the number of
tokens eliminated by compression, matching the `stats` command.

When no compression data exists for a row, the column SHALL display `"-"`.

#### Scenario: cost with compression data

- **WHEN** `copilot-monitor cost` is executed and the store has
  compression-removed token data
- **THEN** the output table SHALL include a `TOKENS_REMOVED` column as the
  rightmost column

#### Scenario: cost without compression data

- **WHEN** no compression-removed tokens exist for any row
- **THEN** the `TOKENS_REMOVED` column SHALL display `"-"` for each row

### Requirement: PolicyPanel heading matches section heading style

The PolicyPanel section heading SHALL use `color: var(--muted)` matching every
other `h2` on the dashboard page.

#### Scenario: Dashboard renders PolicyPanel heading

- **WHEN** the dashboard page is loaded
- **THEN** the "Security Policy" heading SHALL render in `var(--muted)` color,
  visually identical to the "Usage", "Models", "Anomalies", and "Recent
  Sessions" headings

### Requirement: SessionsTable period bar has consistent spacing

The SessionsTable period bar SHALL have zero bottom margin when rendered via the
`<PeriodBar>` component.

#### Scenario: SessionsTable renders period bar

- **WHEN** the dashboard page is loaded and sessions are displayed
- **THEN** the period bar inside the SessionsTable section SHALL have no bottom
  margin, matching its pre-component behavior
