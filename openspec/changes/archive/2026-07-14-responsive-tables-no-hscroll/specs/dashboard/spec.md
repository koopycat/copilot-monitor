## ADDED Requirements

### Requirement: Desktop table fit

The dashboard tables SHALL fit within the viewport width on desktop displays
(>=1024px) without requiring horizontal scrolling. Text-heavy columns (model
names, project labels) MAY truncate with ellipsis when they exceed the available
column width.

#### Scenario: Desktop viewport

- **WHEN** the dashboard is viewed on a display >=1024px wide
- **THEN** the Models and Sessions tables fit within the viewport without a
  horizontal scrollbar

#### Scenario: Model name truncation

- **WHEN** a model name combined with its inline tags exceeds the available
  Model column width
- **THEN** the content is truncated with an ellipsis rather than causing
  horizontal overflow

#### Scenario: Narrow viewport

- **WHEN** the viewport is narrower than 600px
- **THEN** tables may use horizontal scrolling and lower-priority columns (Cache
  %, Token Reduction) are hidden
