## Why

The dashboard has three collapsible sections -- Models, Recent Sessions, and
Anomalies -- but they use two different visual styles. Anomalies has a bordered
card appearance (border, border-radius, surface background, padding) while
Models and Recent Sessions are bare collapsible headers with no container
styling. The inconsistency breaks visual rhythm and makes the sections feel like
they belong to different products.

## What Changes

Apply the bordered card style from the anomaly feed to the `.table-section`
class used by Models and Recent Sessions, making all three collapsible sections
visually consistent.

## Capabilities

### Modified Capabilities

- `dashboard-design-system`: The `.table-section` class SHALL use the same
  bordered card container style as `.anomaly-feed` for visual consistency across
  all collapsible sections.

## Impact

- **CSS only**: `dashboard/src/app.css` -- add `border`, `border-radius`,
  `background`, and `padding` to `.table-section`
- **No markup changes**: Models and Sessions tables remain identical, just
  wrapped in a consistent card container
- **No breaking changes**: Only additive CSS properties
