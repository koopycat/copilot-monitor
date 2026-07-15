## MODIFIED Requirements

### Requirement: Collapsible section visual consistency

All collapsible dashboard sections SHALL use a consistent bordered card
container style: a 1px border in `var(--border)`, border radius of
`var(--radius-lg)`, surface background (`var(--surface)`), and internal padding.

#### Scenario: Models section has card style

- **WHEN** the dashboard loads and the Models section is visible
- **THEN** the section appears as a bordered card matching the Anomalies section
  style

#### Scenario: Sessions section has card style

- **WHEN** the dashboard loads and the Recent Sessions section is visible
- **THEN** the section appears as a bordered card matching the Anomalies section
  style

#### Scenario: Sections remain independently collapsible

- **WHEN** the user toggles any collapsible section
- **THEN** only that section expands or collapses while the card border and
  header remain visible
