## Why

The dashboard's per-model breakdown groups captured requests by
`model, endpoint, upstream_host`. Every proxied request is persisted with
`endpoint = r.URL.Path`, including control-plane / helper calls such as
`GET /models` (model listing) and `GET /agents` (agent listing). These calls
carry no model field and no token usage, so they surface as `<unknown>` model
rows with zero tokens that still inflate request counts and clutter the Models
table, the timeline, the cost estimate inputs, and the CLI `stats` / `cost` /
`today` output. A usage-monitoring tool should report usage, not metadata
discovery traffic, so the system needs an explicit notion of which endpoints are
real model inference versus helper / control-plane traffic.

## What Changes

- Introduce an explicit **endpoint kind** classification: every captured request
  is classified as `inference` (model generation traffic) or `control_plane`
  (helper / metadata traffic such as model or agent listings).
- Classify at capture time and persist the kind alongside the existing
  `endpoint` so it is available to every consumer (reporting, dashboard, export,
  anomalies).
- Model-usage metrics SHALL reflect only `inference` traffic by default:
  - Dashboard Models breakdown, overview request / token counts, projected cost,
    and timeline exclude `control_plane` rows.
  - CLI `stats`, `cost`, and `today` exclude `control_plane` rows.
  - Reporting API `/api/stats`, `/api/cost`, `/api/today`, and
    `/api/stats/timeline` exclude `control_plane` rows.
- Full-fidelity capture is preserved: every request is still stored, still
  visible in raw logs, still available to anomaly detection, and still exported
  via CSV (now with an `endpoint_kind` column).
- No change to request forwarding, streaming, policy enforcement, or session
  boundaries; helper traffic continues to be proxied transparently.

## Capabilities

### New Capabilities

- `endpoint-classification`: Defines how the system classifies each captured
  request endpoint as inference (model generation) or control_plane (helper /
  metadata), and requires every captured request to carry a kind.

### Modified Capabilities

- `capture`: Persisted request rows now include an `endpoint_kind` derived at
  capture time, so classification is durable and queryable.
- `reporting`: The read-only usage views (`stats`, `cost`, `today`, timeline)
  and their backing API endpoints include only `inference` traffic by default;
  CSV export retains all rows and gains an `endpoint_kind` column.
- `dashboard`: The Models breakdown, overview totals, projected cost, and usage
  timeline reflect only `inference` traffic.

## Impact

- **Schema (`internal/store/schema.sql`)**: add a nullable `endpoint_kind`
  column to the `requests` table, backfilled for existing rows (NFR-006: no data
  loss on existing databases). Migration must tolerate pre-existing databases.
- **Proxy (`internal/proxy`)**: classify the request path before persistence and
  pass `endpoint_kind` into the `RequestRecord`.
- **Store + queries (`internal/store`)**: filter model-usage aggregations
  (`Stats`, cost rows, timeline, today) to `endpoint_kind = 'inference'`; add
  the new column to insert and export paths.
- **API (`internal/api`)**: no new endpoints; existing usage endpoints return
  inference-only results. CSV export header gains `endpoint_kind`.
- **CLI (`internal/cli`)**: `stats`, `cost`, `today` reflect inference-only
  data; completion script unaffected.
- **Dashboard (`dashboard/src`)**: no structural change to the Models table; the
  data it already consumes (`/api/stats`, `/api/cost`, timeline) will simply no
  longer contain helper rows.
- **Tests**: new unit tests for the classifier, store filter, and migration;
  integration coverage that a `/models` request does not appear in usage stats.
- **Docs**: update `docs/architecture.md` (capture / classification) and
  `docs/api.md` (inference-only usage views, new CSV column).
