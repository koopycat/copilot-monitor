## ADDED Requirements

### Requirement: Table section disclosure controls

The dashboard SHALL provide independent, keyboard-operable disclosure controls
for the Models and Recent Sessions table sections. Each control SHALL visibly
communicate whether its section is expanded or collapsed and SHALL retain a
visible keyboard focus indicator.

#### Scenario: Keyboard toggle

- **WHEN** a keyboard user focuses a table section disclosure control and
  activates it
- **THEN** only that section changes between expanded and collapsed while the
  other table section retains its state

## MODIFIED Requirements

### Requirement: Per-model breakdown

The dashboard SHALL display a per-model breakdown table showing model names,
request counts, token counts, cache hit ratios, latency, and estimated cost per
model. The Models section SHALL be expanded when the dashboard first loads and
independently collapsible thereafter.

#### Scenario: Models table

- **WHEN** captured data exists and the Models section is expanded
- **THEN** the table shows one row per model with requests, tokens, cache
  percentage, average latency, and cost

#### Scenario: Models section collapsed

- **WHEN** the user collapses the Models section
- **THEN** the model table is hidden while the Models section control remains
  visible

### Requirement: Recent sessions table

The dashboard SHALL display recent sessions in a table with start time,
duration, project label, request count, token count, and estimated cost. The
Recent Sessions section SHALL be expanded when the dashboard first loads and
independently collapsible thereafter.

#### Scenario: Sessions table

- **WHEN** sessions exist in the database and the Recent Sessions section is
  expanded
- **THEN** they are listed in reverse chronological order with summary fields

#### Scenario: Recent Sessions section collapsed

- **WHEN** the user collapses the Recent Sessions section
- **THEN** the sessions table is hidden while the Recent Sessions section
  control remains visible
