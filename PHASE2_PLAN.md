# Phase 2 — Dashboard Improvements (Detailed Plan)

## Goal
Surface upstream host routing, upstream filtering, active proxy routes, and period navigation UX in the dashboard.

---

## Step 1 — Add `upstream_host` to `ModelStats` and `StatsFilter`

**`internal/store/store.go`**:
- Add `UpstreamHost string` to `StatsFilter`
- Add `UpstreamHost string \`json:"upstream_host"\`` to `ModelStats`
- Add `AND (? = '' OR upstream_host = ?)` to `Stats()` WHERE clause
- Add `upstream_host` to `GROUP BY model, endpoint, upstream_host`
- Update `queryModelStats` to scan the new column

## Step 2 — Add index on `upstream_host` and update `Timeline`

**`internal/store/schema.sql`**:
- `CREATE INDEX IF NOT EXISTS idx_requests_upstream_host ON requests(upstream_host);`

**`internal/store/store.go`**:
- Add `upstream_host` filter to `Timeline()` WHERE clause (filter only, don't group by it — keeps chart legible)

## Step 3 — Parse `upstream` query param in API handlers

**`internal/api/api.go`**:
- Add `parseUpstreamParam(r *http.Request) string` helper

**`internal/api/{stats,timeline,today,cost}.go`**:
- Add `UpstreamHost: parseUpstreamParam(r)` to `StatsFilter`

## Step 4 — Add `/api/upstreams` endpoint

**`internal/store/store.go`**:
- Add `DistinctUpstreamHosts(ctx) ([]string, error)` — `SELECT DISTINCT upstream_host FROM requests`

**`internal/api/api.go`**:
- Route `GET /api/upstreams` → returns JSON array

## Step 5 — Add `/api/config` endpoint

**`internal/api/api.go`**:
- `Handler` gains `routesConfig *proxy.ProxyConfig` field
- Route `GET /api/config` → returns active route config as JSON

## Step 6 — Thread config through `serve` command

**`internal/cli/serve.go`**:
- Add `--routes-config` flag
- Load config, pass to `api.NewHandler(st, config)`

**`internal/api/api.go`**:
- Update `NewHandler` signature (backward-compat wrapper keeps existing callers working)

## Step 7 — Dashboard: add `upstream_host` to types and API layer

**`dashboard/src/lib/types.ts`**:
- Add `upstream_host: string` to `ModelStats`

**`dashboard/src/lib/api.ts`**:
- Add `upstream?: string` to `loadDashboard()` params
- Add `fetchUpstreams()` and `fetchConfig()` functions
- Export `RouteConfig` type

## Step 8 — Dashboard: upstream filter dropdown

**`dashboard/src/stores/dashboard.svelte.ts`**:
- Add `upstream` and `upstreams` state fields
- Wire into `load()` flow
- Add `switchUpstream(value)` action

## Step 9 — Dashboard: upstream column in ModelsTable

**`dashboard/src/components/ModelsTable.svelte`**:
- Add `<th>Upstream</th>` column with `<td>{s.upstream_host}</td>`
- Show as tag style, conditionally hidden when empty

## Step 10 — Dashboard: active routes panel

**New: `dashboard/src/components/RoutesPanel.svelte`**:
- Collapsible panel showing path, upstream, capture mode per route

**`dashboard/src/stores/dashboard.svelte.ts`**:
- Add `routes` state, fetch from `/api/config` on init

**`dashboard/src/App.svelte`**:
- Render `<RoutesPanel />` below metric cards

## Step 11 — Dashboard: period navigation improvements

**`dashboard/src/stores/dashboard.svelte.ts`**:
- Keyboard listener: ArrowLeft/ArrowRight navigates periods
- `periodIsEmpty` flag set when stats are empty

**`dashboard/src/components/PeriodBar.svelte`**:
- `aria-current="page"` on active button
- Dot indicator on periods that have data

**`dashboard/src/App.svelte`**:
- Empty-state message when no data for selected period

## Step 12 — Dashboard: upstream filter component

**New: `dashboard/src/components/UpstreamFilter.svelte`**:
- `<select>` dropdown with "All" default, populated from upstreams list

**`dashboard/src/App.svelte`**:
- Render `<UpstreamFilter />` next to period bar

---

## Files Summary

| File | Change |
|---|---|
| `internal/store/schema.sql` | Add upstream_host index |
| `internal/store/store.go` | StatsFilter, ModelStats, Stats GROUP BY, Timeline filter, DistinctUpstreamHosts |
| `internal/api/api.go` | parseUpstreamParam, routesConfig field, /api/upstreams, /api/config |
| `internal/api/stats.go` | UpstreamHost filter |
| `internal/api/timeline.go` | UpstreamHost filter |
| `internal/api/today.go` | UpstreamHost filter |
| `internal/api/cost.go` | UpstreamHost filter |
| `internal/cli/serve.go` | --routes-config flag |
| `dashboard/src/lib/types.ts` | upstream_host, RouteConfig |
| `dashboard/src/lib/api.ts` | upstream param, fetchUpstreams, fetchConfig |
| `dashboard/src/stores/dashboard.svelte.ts` | upstream/upstreams/routes state, keyboard nav, empty-state |
| `dashboard/src/components/ModelsTable.svelte` | Upstream column |
| `dashboard/src/App.svelte` | UpstreamFilter, RoutesPanel, empty-state |
| `dashboard/src/app.css` | New component styles |

## New Files

| File | Purpose |
|---|---|
| `dashboard/src/components/UpstreamFilter.svelte` | Upstream host dropdown |
| `dashboard/src/components/RoutesPanel.svelte` | Active routes display |

## Risks

1. **GROUP BY change**: Stats now groups by `model, endpoint, upstream_host` — frontend merge key must include upstream_host
2. **Timeline**: Filtered but not grouped by upstream_host (keeps chart legible)
3. **`serve` command compat**: `NewHandler` signature change needs backward-compat wrapper
4. **Empty upstream_host in legacy data**: UI must handle gracefully
