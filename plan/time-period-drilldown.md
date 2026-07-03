# Time Period Drill-Down for Dashboard

Status: done

## Goal

Add a time period selector to the dashboard so the user can scope all displayed
data to a chosen window: today, yesterday, last 7 days, last 30 days, last
90 days, last year (365 days). When a period is selected, every section on the
page -- cost summary, model distribution, usage chart, sessions, compare --
must be limited to that period.

## Current state

The dashboard hardcodes `since=30d` for stats, cost, sessions, and the timeline
chart. There is no UI for changing the window.  The backend `parseSinceParam`
already handles `Xd` and Go-duration strings but not ISO timestamps.

## Design

### Period values and their since computation

| Period     | since value                    | Notes |
|---|---|---|
| today      | ISO timestamp, midnight today (local date boundary, UTC) | Requires backend ISO support |
| yesterday  | ISO timestamp, midnight yesterday to midnight today     | Requires `until` or two-ISO approach |
| last 7d    | `7d`                           | Already supported |
| last 30d   | `30d`                          | Already supported |
| last 90d   | `90d`                          | Already supported |
| last 365d  | `365d`                         | Already supported |

"yesterday" is special: it needs a precise range (midnight yesterday to midnight
today), not a rolling 24h window. Two options:

- **Option A (preferred)**: Backend adds `until` query param support to
  `StatsFilter` and relevant queries. Frontend sends `since=...&until=...` for
  yesterday.
- **Option B**: Frontend computes a 24h offset and sends `since=24h`, accepting
  the slight imprecision of a rolling window.

Pick Option A. It's more correct and the backend plumbing is small.

### Backend changes

1. **`parseSinceParam`**: add ISO 8601 timestamp parsing (e.g.,
   `2026-07-03T00:00:00Z`). Keep existing `Xd` and duration support.
   Backward-compatible.

2. **`parseUntilParam`**: new helper, same parsing logic as `parseSinceParam`.

3. **`StatsFilter.Until`**: already exists in the struct. Wire it into the SQL
   queries in `Stats`, `Timeline`, `Sessions`. The column is `ts` (text, ISO)
   so the filter is `ts < ?`.

4. **API handlers**: call `parseUntilParam` and populate `filter.Until` for
   `stats`, `cost`, `today`, `timeline`, `sessions`, `export`.

5. **`/api/today`**: can be kept for backward compatibility, but the dashboard
   will stop using it directly and instead use `/api/stats?since=...` with the
   selected period.

### Frontend changes

1. **Period selector UI**: A button group at the top of the dashboard:
   `Today | Yesterday | 7d | 30d | 90d | 365d`. Active state reflects current
   selection. Default: 30d (current behavior).

2. **`App()` state**: add `period` field (default `'30d'`). When period changes,
   recompute `since` and `until` values and reload all data.

3. **API call updates**: all `safeFetch` calls use the computed `since`/`until`
   from the current period instead of hardcoded values.

4. **Cost summary label**: change from "est. AI-credit cost, 30d" to reflect the
   active period (e.g., "est. AI-credit cost, today").

5. **Projected metric**: adapt based on period. For periods < 30d, project to
   30d. For 365d, show monthly average. For today/yesterday, show as-is without
   projection (or project to 30d with a note).

6. **Compare section**: when period is <= 7d, the monthly comparison is
   misleading. Hide the compare panel for short periods, or switch to a
   day-over-day comparison.

7. **Timeline chart**: the granularity toggle (day/hour) should stay. For
   periods <= 2d, default to hour granularity. For longer periods, default to
   day.

### Files to touch

| File | Change |
|---|---|
| `internal/api/api.go` | Add `parseUntilParam`, wire into handlers |
| `internal/api/stats.go` | Pass `Until` from query param |
| `internal/api/cost.go` | Pass `Until` from query param |
| `internal/api/today.go` | Pass `Until` from query param |
| `internal/api/timeline.go` | Pass `Until` from query param |
| `internal/api/sessions.go` | Pass `Until` from query param |
| `internal/api/export.go` | Pass `Until` from query param |
| `internal/store/store.go` | Wire `StatsFilter.Until` into SQL WHERE clauses |
| `internal/store/sessions.go` | Wire `Until` into sessions query |
| `internal/dashboard/index.html` | Add period selector button group |
| `internal/dashboard/app.js` | Period state, since/until computation, API call updates, label updates |

### Things NOT changing

- The live session panel stays unfiltered (it reflects real-time activity).
- `/api/compare` stays month-based for now; it may be hidden for short periods.
- CLI commands (`stats`, `cost`, `today`) are unaffected; the `--since` flag
  already works there.

### Testing

- Backend: add test cases for ISO timestamp parsing in `parseSinceParam`.
- Backend: add test cases for `until` filter in store queries (check that rows
  after `until` are excluded).
- Frontend: manual smoke test each period, verify all sections update.
- Run `just test` and `just all` before completing.

### Open questions

1. For "yesterday", should we use local-time midnight or UTC midnight? Lean
   toward local time (consistent with existing `/api/today` behavior).
2. Compare panel for short periods: hide it, or show day-over-day? Start by
   hiding it for today/yesterday, keep it for >= 7d.
3. Should the period selection persist across page reloads? Start without
   persistence; always default to 30d on load.
