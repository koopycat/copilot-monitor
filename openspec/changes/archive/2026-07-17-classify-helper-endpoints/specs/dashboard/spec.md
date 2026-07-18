## ADDED Requirements

### Requirement: Inference-only dashboard usage views

The dashboard's usage views SHALL reflect only model-generation traffic
(requests with `endpoint_kind` of `inference`). This SHALL apply to the overview
total request count and projected cost, the per-model breakdown table, and the
usage timeline chart. Requests with `endpoint_kind` of `control_plane`, such as
`GET /models` and `GET /agents`, SHALL NOT appear as model rows, SHALL NOT be
counted in the overview request total, and SHALL NOT contribute tokens or cost
to any dashboard view.

#### Scenario: Models table excludes model listing

- **WHEN** the captured data contains a `GET /models` request classified as
  `control_plane`
- **THEN** no row for `/models` or the `<unknown>` model appears in the
  per-model breakdown table

#### Scenario: Models table excludes agent listing

- **WHEN** the captured data contains a `GET /agents` request classified as
  `control_plane`
- **THEN** no row for `/agents` or the `<unknown>` model appears in the
  per-model breakdown table

#### Scenario: Overview counts only inference requests

- **WHEN** the overview metrics are computed for a period that includes both
  `inference` and `control_plane` requests
- **THEN** the total request count and projected cost reflect only the
  `inference` requests

#### Scenario: Timeline excludes control-plane traffic

- **WHEN** the usage timeline is rendered for a period that includes
  `control_plane` requests
- **THEN** the timeline buckets contain only `inference` requests

#### Scenario: Inference models still shown

- **WHEN** the captured data contains `inference` requests for one or more
  models
- **THEN** those models appear in the per-model breakdown as before
