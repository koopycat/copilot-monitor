## MODIFIED Requirements

### Requirement: Collapsible section visual consistency

All collapsible dashboard sections SHALL use the native `<details>` disclosure
arrow as the sole collapse indicator. No section SHALL display custom
"collapsed"/"expanded" text labels.

#### Scenario: Models section uses arrow only

- **WHEN** the dashboard loads and the Models section is visible
- **THEN** the section shows only the native disclosure arrow as its collapse
  indicator

#### Scenario: Sessions section uses arrow only

- **WHEN** the dashboard loads and the Recent Sessions section is visible
- **THEN** the section shows only the native disclosure arrow as its collapse
  indicator

#### Scenario: All sections use the same indicator

- **WHEN** the dashboard is fully loaded
- **THEN** all collapsible sections (Models, Sessions, Anomalies, Routes) show
  identical collapse indicators
