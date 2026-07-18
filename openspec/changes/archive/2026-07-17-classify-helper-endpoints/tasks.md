## 1. Endpoint classifier

- [x] 1.1 Add a
      `ClassifyEndpointKind(path string, hasModel bool, hasUsage bool) string`
      helper in `internal/proxy` (next to `model_discovery.go`) returning
      `"inference"` or `"control_plane"`: normalize a leading `/v1/`,
      `/copilot/`, or `/openai/` prefix and a trailing slash; treat `/models`
      and `/agents` as `control_plane`; otherwise `control_plane` when
      `!hasModel && !hasUsage`, else `inference`.
- [x] 1.2 Add unit tests in `internal/proxy` covering every spec scenario:
      `/models`, `/agents`, prefixed variants, `/chat/completions` with usage,
      `/responses` with model, unknown path with neither, unknown path carrying
      a model.

## 2. Schema and migration

- [x] 2.1 Add `endpoint_kind TEXT` column to the `requests` table in
      `internal/store/schema.sql`.
- [x] 2.2 Add an idempotent migration in the store open path:
      `ALTER TABLE requests ADD COLUMN endpoint_kind TEXT` if absent, then
      backfill `NULL` rows from `endpoint` (best-effort model/usage signal from
      existing columns).
- [x] 2.3 Add a store test that opens a pre-existing database without the
      column, confirms rows are backfilled with a non-null `endpoint_kind`, and
      that no other data is lost.

## 3. Store persistence and queries

- [x] 3.1 Add `EndpointKind string` to `store.RequestRecord` and include it in
      `InsertRequest`.
- [x] 3.2 Add `AND endpoint_kind = 'inference'` to the model-usage aggregations:
      `Stats`, the cost-rows query, `Timeline`, and the `today` query in
      `internal/store`.
- [x] 3.3 Add an `endpoint_kind` column to the CSV export header and row writer
      in `internal/api/export.go` (no kind filter on export).
- [x] 3.4 Add store tests: a `control_plane` row is excluded from
      `Stats`/cost/timeline/today but present in export; an `inference` row with
      `usage_missing` is still included.

## 4. Capture wiring

- [x] 4.1 In `internal/proxy/server.go`, compute `endpoint_kind` from
      `r.URL.Path`, the parsed request model (`meta.Model`), and the observer
      usage signal; pass it into every `store.RequestRecord` (normal,
      blocked-403, and WebSocket persist paths).
- [x] 4.2 Add a proxy test capturing `GET /models` and `GET /agents` and
      asserting the persisted rows have `endpoint_kind = "control_plane"`.

## 5. Reporting API and CLI

- [x] 5.1 Add an integration test that captures a `GET /models` request plus an
      inference request, then asserts `/api/stats`, `/api/cost`, `/api/today`,
      and `/api/stats/timeline` contain only the inference row.
- [x] 5.2 Verify CLI `stats`, `cost`, and `today` reflect inference-only data
      and that `export` emits the `endpoint_kind` column with both kinds
      present.

## 6. Documentation and verification

- [x] 6.1 Update `docs/architecture.md` (endpoint classification at capture;
      inference-only usage views) and `docs/api.md` (usage endpoints are
      inference-only; CSV export gains `endpoint_kind`).
- [x] 6.2 Run `just all` (vet, secret scan, fast unit tests, full build) and
      resolve any failures, including flakiness not caused by this change.
