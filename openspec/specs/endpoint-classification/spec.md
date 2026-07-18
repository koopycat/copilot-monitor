<!-- markdownlint-disable MD041 -->

## Purpose

Classify every captured proxied request endpoint as either model-generation
(`inference`) or helper/control-plane (`control_plane`) traffic so that usage
views, cost estimates, and exports can distinguish real model usage from
discovery and metadata requests.

## Requirements

### Requirement: Endpoint kind classification

The system SHALL assign every captured proxied request an `endpoint_kind` of
`inference` (model-generation traffic) or `control_plane` (helper / discovery /
metadata traffic that is not model generation). Classification SHALL be based on
the request path, normalized by stripping a single leading `/v1/`, `/copilot/`,
or `/openai/` prefix and any trailing slash. A request SHALL be classified
`control_plane` when its normalized path is a known helper listing (`/models`,
`/agents`) OR when it carries no model field and no captured token usage;
otherwise it SHALL be classified `inference`.

#### Scenario: Model listing is control plane

- **WHEN** a `GET /models` request is proxied
- **THEN** its captured row has `endpoint_kind` of `control_plane`

#### Scenario: Agent listing is control plane

- **WHEN** a `GET /agents` request is proxied
- **THEN** its captured row has `endpoint_kind` of `control_plane`

#### Scenario: Prefixed model listing is control plane

- **WHEN** a `GET /v1/models` or `GET /copilot/models` request is proxied
- **THEN** its captured row has `endpoint_kind` of `control_plane`

#### Scenario: Chat completion is inference

- **WHEN** a `POST /chat/completions` request is proxied and the response
  contains token usage
- **THEN** its captured row has `endpoint_kind` of `inference`

#### Scenario: Responses endpoint is inference

- **WHEN** a `POST /responses` request is proxied and carries a model field
- **THEN** its captured row has `endpoint_kind` of `inference`

#### Scenario: Unknown path with no model and no usage is control plane

- **WHEN** a request to an unrecognized path is proxied and the request carries
  no model field and the response carries no token usage
- **THEN** its captured row has `endpoint_kind` of `control_plane`

#### Scenario: Unknown path carrying a model is inference

- **WHEN** a request to a previously unseen path is proxied and the request body
  names a model
- **THEN** its captured row has `endpoint_kind` of `inference`

---

### Requirement: Endpoint kind persistence

The system SHALL persist `endpoint_kind` on every captured request row, SHALL
backfill `endpoint_kind` for rows captured before this capability existed, and
SHALL expose the field through the same name (`endpoint_kind`) in the reporting
API and CSV export.

#### Scenario: New capture stores the kind

- **WHEN** a request is captured after this capability ships
- **THEN** the persisted `requests` row has a non-null `endpoint_kind`

#### Scenario: Historical rows are backfilled

- **WHEN** the store opens a database that contains rows without an
  `endpoint_kind`
- **THEN** those rows are assigned an `endpoint_kind` derived from their stored
  `endpoint` without losing any other data

#### Scenario: Kind available in export

- **WHEN** a CSV export is produced
- **THEN** each row includes an `endpoint_kind` column
