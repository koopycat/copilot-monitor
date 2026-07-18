## ADDED Requirements

### Requirement: Inference-only usage aggregation

Model-usage reporting SHALL include only requests whose `endpoint_kind` is
`inference`. This SHALL apply to the dashboard API endpoints `/api/stats`,
`/api/cost`, `/api/today`, and `/api/stats/timeline`, and to the CLI `stats`,
`cost`, and `today` subcommands. Requests with `endpoint_kind` of
`control_plane` (for example `GET /models` and `GET /agents`) SHALL be excluded
from these usage views so that request counts, token totals, cost estimates, and
timeline buckets reflect model-generation traffic only.

#### Scenario: Stats exclude model listing

- **WHEN** the database contains a `GET /models` request captured as
  `control_plane` and `copilot-monitor stats` is run
- **THEN** the `/models` request does not appear in the stats output and is not
  counted in any model or endpoint total

#### Scenario: Cost excludes agent listing

- **WHEN** the database contains a `GET /agents` request captured as
  `control_plane` and the cost view is computed
- **THEN** the `/agents` request contributes no cost row and is not counted in
  the request total

#### Scenario: Timeline excludes control-plane traffic

- **WHEN** the timeline is computed for a period that includes control-plane
  requests
- **THEN** the timeline buckets contain only `inference` requests

#### Scenario: Today view excludes control-plane traffic

- **WHEN** `copilot-monitor today` is run and the period includes
  `control_plane` requests
- **THEN** only `inference` requests since local midnight are reported

#### Scenario: Usage-missing inference rows are still included

- **WHEN** an `inference` request has `usage_missing` set
- **THEN** it is still included in usage views and is subject to the existing
  usage-missing footnote

## MODIFIED Requirements

### Requirement: CSV export

CSV export SHALL include stored metadata fields for rows meeting export filters
and omit bodies and secrets. Export SHALL include rows of every `endpoint_kind`
and SHALL emit an `endpoint_kind` column so control-plane traffic remains
visible in full-fidelity exports.

#### Scenario: CLI export

- **WHEN** `copilot-monitor export --since 30d` is run
- **THEN** a CSV with header row and metadata columns is written to stdout

#### Scenario: API export

- **WHEN** a GET request is made to `/api/export`
- **THEN** a CSV with Content-Disposition header is returned

#### Scenario: Export includes endpoint kind

- **WHEN** a CSV export is produced and the exported rows include both
  `inference` and `control_plane` requests
- **THEN** every row has an `endpoint_kind` column and control-plane rows are
  present in the output
