## Context

copilot-monitor currently rebuilds the `sessions` table on every read by
scanning all rows from `requests`, sorting by timestamp, and splitting on
30-minute gaps. This runs from `/api/sessions`, `/api/session/current`, CLI
`sessions`, and CLI `live --watch` (every 2 seconds). As data grows beyond a few
thousand requests, this O(n) scan dominates every refresh cycle. Separately, the
dashboard UI hard-codes a 20-session limit with no pagination or filtering, the
usage chart has a 7-color ceiling, and anomalies are collected but invisible.

## Goals / Non-Goals

**Goals:**

- Eliminate full-table-scan session rebuilds from the read hot-path
- Maintain correct session grouping (30-min gap rule) during incremental writes
- Add pagination and filtering to the dashboard sessions view
- Make the chart legend work with an unbounded number of models
- Expose stored anomalies via API and dashboard

**Non-Goals:**

- Changing the session gap heuristic (stays at 30 minutes, now configurable via
  CLI)
- Database engine migration (stays on SQLite)
- Custom date-range picker in the dashboard (deferred to follow-up)
- Canvas hit-testing / bar-level drill-down (deferred to follow-up)
- Prometheus metrics endpoint (not needed for single-user desktop tool)

## Decisions

### Decision 1: Session tracking strategy

**Chosen:** Anchor on `MAX(ended_at)` from the `sessions` table. On each
`InsertRequest`, compare the new request's timestamp against the latest
session's `ended_at`. If the gap is < 30 minutes, join; otherwise, create a new
session.

**Alternatives considered:**

- **Track last request ID instead of timestamp.** Fragile across process
  restarts since IDs reset if the DB is rebuilt.
- **Use a dedicated `current_session_id` state variable in the Store struct.**
  Would not survive process restart. DB-anchored approach is stateless.

**Rationale:** Querying `MAX(ended_at)` on `sessions` is O(log n) via the
implicit B-tree on the primary key, so it adds negligible cost. The lookup
happens inside the same write transaction as the INSERT, ensuring atomicity.

**Out-of-order handling:** If a request arrives with a timestamp _older_ than
the latest session's `ended_at` but within the 30-minute gap of that session, it
is still assigned to that session. `request_count` and `token_count` are
incremented; `ended_at` is not moved backward. The `rebuild-sessions` CLI
command provides a full re-sort-and-rebuild for repair.

### Decision 2: Session summary atomicity

**Chosen:** The `InsertRequest` implementation wraps the request INSERT and
session UPDATE in a single SQLite transaction (BEGIN/COMMIT). The transaction
reads `MAX(ended_at)`, performs the INSERT with `session_id`, and updates
`request_count`, `token_count`, and `ended_at` on the sessions row.

**Rationale:** `database/sql` transactions with modernc.org/sqlite in WAL mode
provide ACID guarantees for concurrent readers. WAL readers see a consistent
snapshot, so dashboard queries never see a half-updated session.

### Decision 3: Session model stats batching

**Chosen:** Replace the per-session `WHERE session_id = ?` loop in
`handleSessions` with a single `WHERE session_id IN (?, ?, ...)` query, grouped
by session_id + model + endpoint.

**Rationale:** For 20 sessions this avoids 19 round-trips. The query builds a
dynamic IN clause (safe: session IDs are integers, SQLite has no parameter count
limit for practical session counts).

### Decision 4: Chart model grouping

**Chosen:** On the client side, compute each model's share of total tokens (or
requests). If the share < 5%, pool it into "Other." The client-side stats
already have per-model token counts, so this is a pure rendering decision.

**Rationale:** Avoids adding complexity to the timeline API. The threshold is
configurable via a constant. 5% was chosen because it typically groups 0-2
long-tail models for a user with 5-10 models, and 10-15 models for a heavier
user with 30+ models.

### Decision 5: HSL color interpolation

**Chosen:** Replace the hardcoded 7-color palette with HSL-based interpolation:
`hsl((i * 360 / count), 65%, 55%)`. This distributes hues evenly around the
wheel.

**Alternatives considered:**

- **Maintain a hash-based color map.** Same problem -- collision-prone for many
  models.
- **Use a perceptually-uniform color space (OKLCH).** Better but adds complexity
  and the existing CSS already uses OKLCH. A follow-up can switch the chart to
  OKLCH once the color infrastructure is in place.

**Rationale:** HSL is simple, widely supported, and produces visually distinct
colors for practical model counts (up to ~50). No library needed.

### Decision 6: Session pagination

**Chosen:** Cursor-based pagination using `(started_at, id)` tuples. The API
accepts `?cursor=<started_at>&cursor_id=<id>` and returns the next page after
that cursor.

**Alternatives considered:**

- **Offset-based pagination.** Simpler but breaks when rows are inserted between
  pages (e.g., during auto-refresh). Cursor-based pagination is stable.

**Rationale:** The `sessions` table already has an implicit index on the primary
key `id` and ordering by `started_at DESC, id DESC` is efficient. 20 sessions
per page.

### Decision 7: Retention pruning

**Chosen:** Batch deletes in chunks of 1000 with a configurable sleep between
batches (default 100ms). Session boundary handling: sessions that straddle the
cutoff are retained entirely (no partial deletion).

**Rationale:** 1000-row batches complete quickly enough (<10ms on SSD) to not
block proxy writes. The boundary rule avoids corrupting session summaries.

### Decision 8: No automatic VACUUM

**Chosen:** `VACUUM` is only available via the manual
`copilot-monitor rebuild-sessions --vacuum` command.

**Rationale:** `VACUUM` rewrites the entire database file, locking it for the
duration. On a 2GB file this can take minutes. For a desktop tool, this is
unacceptable to run automatically. Users get the disk space back only when they
explicitly choose to.

### Decision 9: `catalog.LoadDefault()` caching

**Chosen:** Cache the catalog in the Store struct with `sync.Once`, loaded on
first access. The catalog is immutable at runtime (embedded in the binary), so
this is safe.

**Rationale:** Currently re-read, re-parsed, and re-validated on every API call
that needs pricing (3x per dashboard refresh). A `sync.Once` cache eliminates
this entirely with a one-line change.

### Decision 10: PRAGMA tuning

**Chosen:** Add to `store.init`:

- `PRAGMA synchronous=NORMAL` (safe under WAL, reduces fsync pressure)
- `PRAGMA mmap_size=268435456` (256MB memory-mapped I/O for read-heavy workload)
- `PRAGMA temp_store=MEMORY` (GROUP BY spilling uses memory, not disk)

**Rationale:** These are standard SQLite tuning pragmas with no data-loss risk
under WAL mode. The mmap size of 256MB is well within modern desktop memory
budgets. `synchronous=NORMAL` trades a tiny corruption window on OS crash for
significantly reduced write latency -- acceptable for usage monitoring data.

## Risks / Trade-offs

- **[Out-of-order insert] → Session mis-assignment.** Late-arriving requests
  could be placed in the wrong session. Mitigation: `rebuild-sessions` CLI
  command for repair. The proxy inserts with `time.Now()` synchronously in
  normal operation; out-of-order arrivals are rare.

- **[Timestamp migration] → Data loss.** TEXT→INTEGER migration is the
  highest-risk schema change. Mitigation: gated behind benchmarks. If
  implemented, use a reversible approach (add `ts_int`, backfill, keep `ts` as
  nullable, migrate Go code to read/write both, drop `ts` only after a
  deprecation period). Never drop-and-rename in a single step.

- **[Retention pruning] → Accidental data loss.** A misconfigured
  `--retention-days` or clock skew could delete desired data. Mitigation:
  `--retention-days 0` disables pruning entirely. A warning is emitted when
  pruning would delete >50% of the database. A `--dry-run` flag prints what
  would be deleted without executing.

- **[RebuildSessions removal] → Broken CLI `live` / `run` live tail.** These
  currently call `RebuildSessions`. Mitigation: they switch to reading the
  incrementally-maintained sessions table, same as the dashboard API.
