## 1. DB Performance Tuning (Phase 0)

- [x] 1.1 Add PRAGMA tuning to `store.init`: `synchronous=NORMAL`,
      `mmap_size=268435456`, `temp_store=MEMORY`
- [x] 1.2 Cache `catalog.LoadDefault()` with `sync.Once` in the Store struct,
      eliminate per-request re-parse
- [x] 1.3 Add `idx_requests_stats` composite index on `requests(ts, project)`
- [x] 1.4 Drop dead `bodies` table from schema (zero INSERTs in codebase --
      verify with grep)
- [x] 1.5 Remove unused single-column indexes on `requests` that are redundant
      with the composite index

## 2. Incremental Session Tracking (Phase 1)

- [x] 2.1 Implement `Store.assignSession(ctx, tx, rec)` that reads
      `MAX(ended_at)` from sessions, compares gap, and joins or creates a
      session within the same write transaction
- [x] 2.2 Modify `InsertRequest` to call `assignSession` atomically (request
      INSERT + session UPDATE in one tx)
- [x] 2.3 Remove `RebuildSessions` calls from `/api/sessions`,
      `/api/session/current`, CLI `sessions`, CLI `live`, and `run` live tail
- [x] 2.4 Create `copilot-monitor rebuild-sessions` CLI subcommand for offline
      full reconstruction
- [x] 2.5 Add `--gap` flag to `rebuild-sessions` (default 30m) and `--vacuum`
      flag for optional VACUUM after rebuild
- [x] 2.6 Add Store tests for incremental session assignment (new request joins
      existing, new request creates new, out-of-order timestamp)
- [x] 2.7 Add Store tests for `rebuild-sessions` correctness against a known set
      of requests

## 3. Data Retention (Phase 3)

- [x] 3.1 Add `--retention-days` flag to `run` and `serve` commands (default
      365, 0 disables)
- [x] 3.2 Add `--anomaly-retention-days` flag (default 30)
- [x] 3.3 Implement `Store.PruneRequests(ctx, before)` with batched deletes
      (1000 rows per batch, yield between batches)
- [x] 3.4 Implement session-boundary-aware pruning (sessions straddling the
      cutoff are retained entirely)
- [x] 3.5 Run prune on startup and every 24h via a background goroutine
- [x] 3.6 Add `--dry-run` flag that reports what would be deleted without
      executing
- [x] 3.7 Emit warning when pruning would delete >50% of database rows
- [x] 3.8 Include `retention_days`, `last_prune_at`, `pruned_count` in
      `/api/health` response
- [x] 3.9 Add retention tests (boundary session, batched delete, dry-run)

## 4. API Optimizations (Phase 4)

- [x] 4.1 Batch session model stats: single `WHERE session_id IN (...)` query
      replacing the N+1 loop in `handleSessions`
- [x] 4.2 Sanitize API error responses: structured JSON
      `{"error": "internal server error"}` with full error logged server-side
- [x] 4.3 Add cursor-based pagination to `/api/sessions`
      (`?cursor=<started_at>&cursor_id=<id>`)
- [x] 4.4 Add `/api/sessions/count` endpoint returning total session count
- [x] 4.5 Add `/api/anomalies` endpoint (wraps existing `QueryAnomalies`, add
      LIMIT 50, support `?category=` and `?severity=` params)
- [x] 4.6 Add `/api/sessions/distinct-projects` endpoint returning distinct
      project names
- [x] 4.7 Add refresh guard in dashboard store: skip timer tick if
      `this.loading` is true

## 5. Dashboard -- Session Scaling (Phase 5)

- [x] 5.1 Replace hardcoded `limit=20` with 20-per-page cursor-based pagination
      in `api.ts`
- [x] 5.2 Add "Load more" button to SessionsTable component, appending pages
- [x] 5.3 Add project filter dropdown to sessions view (populated from
      `/api/sessions/distinct-projects`)
- [x] 5.4 Add date-range filter (since/until) to sessions view using existing
      PeriodBar-style presets
- [x] 5.5 Show session count indicator: "Showing 1-20 of 847 sessions"

## 6. Dashboard -- Chart & Model Scaling (Phase 6)

- [x] 6.1 Implement "Other" grouping in chart: pool models below 5% of total
      into aggregate category
- [x] 6.2 Replace hardcoded 7-color palette with HSL hue interpolation for N
      distinct colors
- [x] 6.3 Add click-to-sort on ModelsTable column headers (model, requests,
      tokens, latency, cost)
- [x] 6.4 Add sort direction indicator (▲/▼) on the active sort column

## 7. Dashboard -- Anomaly Visibility (Phase 7)

- [x] 7.1 Add `fetchAnomalies()` to `api.ts` calling `/api/anomalies`
- [x] 7.2 Add anomaly state to `dashboard.svelte.ts` (loading during each
      refresh)
- [x] 7.3 Create `AnomalyFeed.svelte` component: collapsible section, 10 most
      recent, severity badges
- [x] 7.4 Add empty state: "No anomalies detected" when feed is empty

## 8. Dashboard -- Polish (Phase 8)

- [x] 8.1 Add loading spinner/pulse on refresh button during data fetch
- [x] 8.2 Skip auto-refresh when previous refresh is still in flight (guard in
      timer)
- [x] 8.3 Add brief "Updated" timestamp flash animation on new data arrival
- [x] 8.4 Fix "projected this month" label: show "today so far" / "yesterday" /
      "projected this month" per period
- [x] 8.5 Add upstream-filter-active indicator badge when filter is set

## 9. Observability & Edge Cases

- [x] 8.6 Cache `catalog.LoadDefault()` result in `api.Handler` (avoid re-parse
      on every request)
- [x] 8.7 Ensure `/api/health` does not trigger full table scans (remove
      unfiltered `Stats` call, use `COUNT(*)` instead)
- [x] 8.8 Add Store-level connection pooling limits: `db.SetMaxOpenConns(1)` for
      write serialization
