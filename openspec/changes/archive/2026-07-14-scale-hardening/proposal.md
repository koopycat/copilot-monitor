## Why

The copilot-monitor dashboard and CLI become progressively slower as users
accumulate months of usage data. The session reconstruction query reads every
request row on every refresh, TEXT timestamps force per-row string parsing for
time bucketing, and the dashboard lacks pagination, filtering, and anomaly
visibility -- making it progressively less useful as data grows beyond a few
weeks.

## What Changes

- **Incremental session tracking**: Assign `session_id` during `InsertRequest`
  instead of rebuilding the entire sessions table on every read. Keep a CLI
  `rebuild-sessions` command for recovery and schema migrations.
- **DB performance hardening**: Add PRAGMA tuning (mmap, temp_store), cache the
  pricing catalog, and add a composite index on `(ts, project)`. Investigate an
  INTEGER timestamp migration (gated behind benchmarks).
- **Data retention**: Configurable `--retention-days` flag with batched, safe
  pruning of old requests, sessions, and anomalies. No automatic VACUUM;
  compaction is manual.
- **Dashboard session scaling**: Add pagination (cursor-based), project filter,
  and date-range filter for sessions. Expose total session count.
- **Dashboard chart scaling**: Group long-tail models into "Other" category in
  the usage chart. Use HSL color interpolation for unbounded model colors.
- **Anomaly visibility**: New `/api/anomalies` endpoint and a dashboard anomaly
  feed showing recent warnings and errors.
- **API optimization**: Batch session-model queries (N+1 -> single query), add
  refresh guard against cascading reloads, cache the pricing catalog with
  `sync.Once`.

## Capabilities

### New Capabilities

- `session-tracking`: Incremental session assignment at request-insertion time,
  replacing the O(n) full-table rebuild. Sessions are maintained per-write
  instead of recomputed per-read.
- `data-retention`: Configurable TTL-based pruning of old requests, sessions,
  and anomalies. Safe batched deletes with no automatic VACUUM.
- `anomaly-visibility`: API endpoint and dashboard feed for proxy-detected
  anomalies (unrouted paths, auth failures, parse errors).

### Modified Capabilities

- `dashboard`: Adds session pagination/filtering, model-chart grouping,
  model-table sorting, anomaly feed, and refresh feedback. Chart color palette
  scales beyond 7 models.
- `reporting`: Merges duplicate Stats/Cost queries. Adds `/api/anomalies`
  endpoint. Batches session-model queries. Sanitizes error responses.

## Impact

- **DB schema**: New composite index `idx_requests_stats(ts, project)`.
  Migration to add `ts_int` INTEGER column if benchmarked necessary. `bodies`
  table dropped (never used by any code path).
- **Store API**: `InsertRequest` gains session-assignment logic.
  `RebuildSessions` removed from hot path, becomes CLI-only. New
  `PruneRequests`, `QueryAnomalies` (already exists, needs API exposure),
  `SessionCount`, and cursor-based session pagination.
- **API**: New `/api/anomalies` endpoint. `/api/sessions` gains cursor-based
  pagination. Dashboard `/api/stats` + `/api/cost` may merge. Error messages
  sanitized. Refresh guard in frontend timer.
- **Dashboard**: Sessions table gains pagination, project filter, date filter.
  Usage chart gains "Other" grouping and HSL colors. Models table gains
  click-to-sort. Anomaly feed component added. Refresh button shows loading
  state.
- **CLI**: `copilot-monitor rebuild-sessions` becomes a standalone command.
  `run` and `live` commands stop calling `RebuildSessions` on the hot path; they
  read the incrementally-maintained sessions table.
- **Breaking**: None. All API changes are additive. Existing DB files are
  forward-compatible (new columns and indexes only).
