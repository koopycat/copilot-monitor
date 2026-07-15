## MODIFIED Requirements

### Requirement: Per-model breakdown

The dashboard SHALL display a per-model breakdown table showing model names,
request counts, token counts, cache hit ratios, latency, and estimated cost per
model. The table SHALL support sorting by clicking column headers. When more
than five model rows are available, the table SHALL initially display the first
five rows under the active sort and SHALL provide a keyboard-operable control to
show all rows and return to the five-row view. The Models section SHALL be
expanded when the dashboard first loads and independently collapsible
thereafter.

#### Scenario: Models table row content

- **WHEN** captured data exists and the Models section is expanded
- **THEN** each displayed row shows its model name, requests, tokens, cache
  percentage, average latency, and cost, and the complete row set is sorted by
  total tokens descending by default

#### Scenario: Models table with more than five rows

- **WHEN** more than five model rows exist and the Models section is expanded
  for the first time
- **THEN** the table shows the five rows with the highest total-token counts and
  a control labeled `Show all N models`, where N is the complete model-row count

#### Scenario: Models table with five or fewer rows

- **WHEN** five or fewer model rows exist and the Models section is expanded
- **THEN** the table shows every model row and does not show a row disclosure
  control

#### Scenario: Show all models

- **WHEN** the user activates the `Show all N models` control
- **THEN** the table shows every model row and the control changes to
  `Show top 5`

#### Scenario: Return to top five models

- **WHEN** the user activates the `Show top 5` control
- **THEN** the table shows only the first five rows under the active sort and
  the control changes to `Show all N models`

#### Scenario: Sort models by column

- **WHEN** the user clicks a column header such as `Total $`
- **THEN** the complete model-row set is sorted by that column before the
  current five-row or all-row display limit is applied, the selected row
  disclosure state is retained, and clicking the header again reverses the sort
  order

#### Scenario: Visual encoding remains stable

- **WHEN** the user switches between the five-row and all-row views
- **THEN** the inline bar width and model color of every row present in both
  views remain unchanged

#### Scenario: Row disclosure survives data refresh

- **WHEN** the user selects the all-row view and a dashboard refresh still
  returns more than five model rows
- **THEN** the all-row view remains selected and shows the complete refreshed
  model-row set

#### Scenario: Row disclosure is independent from section disclosure

- **WHEN** the user changes the row disclosure state and then collapses and
  reopens the Models section
- **THEN** the selected five-row or all-row state is retained while the Models
  section is hidden and restored

#### Scenario: Models section collapsed

- **WHEN** the user collapses the Models section
- **THEN** the model table and its row disclosure control are hidden while the
  Models section control remains visible
