# Advanced Analytics Implementation Plan

Three features, ordered by value-to-effort ratio.

## Feature 1: Export (CSV / JSON)

### Value

After a year of data, users want to graph their usage in external tools.
A spreadsheet is the universal analysis tool.
Export makes the data portable.

### Design

One CLI command, one API endpoint.
Both reuse the existing `SELECT * FROM requests` query.

```
copilot-monitor export [--format csv|json] [--since 30d] [--db path] [--project x]
```

### Endpoint

```
GET /api/export?format=csv&since=30d
GET /api/export?format=json&since=30d&project=my-project
```

CSV output uses headers matching the JSON field names (`model`, `endpoint`, `prompt_tokens`, ...).
JSON output is a JSON array of request objects.

### Store changes

One new method:

```go
func (s *Store) ExportRequests(ctx context.Context, since time.Time) ([]ExportRow, error)
```

Returns all request rows with their token counts, model, endpoint, timestamp, latency, and status.
Filters by `since` only; no aggregation.

### Implementation order

1. Add `ExportRequests` to `internal/store/store.go` (30 lines)
2. Add `/api/export` handler to `internal/api/api.go` (40 lines)
3. Add `copilot-monitor export` to `internal/cli/cli.go` (60 lines)
4. Add dashboard download button (20 lines HTML)
5. Tests (40 lines)

---

## Feature 2: Period Comparison

### Value

The single most-requested feature after a year of data.
"Am I spending more this month than last month?"
"Is my Claude usage growing?"

### Design

Compare two time periods side by side.
Default: this month vs last month.
Configurable: any two periods via `?a=2026-06&b=2026-07`.

### CLI

```
copilot-monitor compare [--a 2026-06] [--b 2026-07] [--db path]
```

Output: a table with model rows and delta columns.

```
MODEL              JUN COST   JUL COST   DELTA     JUN TOK   JUL TOK   DELTA
claude-sonnet-4.6  $3.50      $4.20      +20%      45K       52K       +16%
gpt-5-mini         $1.20      $0.80      -33%      18K       10K       -44%
TOTAL              $4.70      $5.00      +6%       63K       62K       -2%
```

### API

```
GET /api/compare?a=2026-06&b=2026-07
GET /api/compare?periods=2&bucket=month   (last 2 months)
GET /api/compare?periods=12&bucket=month  (last 12 months)
```

Response:

```json
{
  "periods": [
    {"label": "2026-06", "start": "...", "end": "...", "models": [...], "total_cost": 4.70, "total_tokens": 63000},
    {"label": "2026-07", "start": "...", "end": "...", "models": [...], "total_cost": 5.00, "total_tokens": 62000}
  ]
}
```

### Store changes

```go
func (s *Store) Compare(ctx context.Context, aStart, aEnd, bStart, bEnd time.Time) ([]ComparePeriod, error)
```

Reuses the existing `Stats` query but run for two time windows.

### Dashboard changes

Add a "Compare" section below the chart.
Two-column layout showing this month vs last month metrics.
Small sparklines for each model showing direction.
Toggle to switch between months.

### Implementation order

1. Add `Compare` to store (60 lines)
2. Add `/api/compare` handler (50 lines)
3. Add `copilot-monitor compare` CLI (80 lines)
4. Dashboard comparison panel (80 lines HTML + JS)
5. Tests (60 lines)

---

## Feature 3: Live Session View

### Value

When using Copilot in a running session, the user wants to see a **live cost ticker**.
Not "what happened in the last 30 days" but "what is happening right now."

### Design

A real-time-aware panel on the dashboard.
Uses the existing 30-second refresh cycle but adds a **current session** query.
The current session is the most recent session (by `started_at`).
If the last request was less than 30 minutes ago, the session is considered active.

### Dashboard panel

```
┌─ Live Session ──────────────────────┐
│  Cost     $0.17    (running)        │
│  Requests 5                          │
│  Tokens   3,241                     │
│  Duration 4m 12s                     │
│  Models   gpt-5-mini (4)            │
│           claude-sonnet-4.6 (1)     │
└─────────────────────────────────────┘
```

Pulses green when a new request arrives.
Shows `(idle 12m ago)` when the session is inactive.

### API

```
GET /api/session/current
```

Response:

```json
{
  "session": {
    "id": 42,
    "started_at": "...",
    "request_count": 5,
    "token_count": 3241,
    "cost": 0.17,
    "active": true,
    "last_request_at": "..."
  },
  "models": [
    {"model": "gpt-5-mini", "requests": 4, "tokens": 1200, "cost": 0.08},
    {"model": "claude-sonnet-4.6", "requests": 1, "tokens": 2041, "cost": 0.09}
  ]
}
```

### Store changes

```go
func (s *Store) CurrentSession(ctx context.Context) (*CurrentSession, error)
```

Finds the latest session and calculates its stats.
Returns nil if no session exists or the latest session ended more than 30 minutes ago.

### Cost ticker behavior

- When `active: true`: shows green pulse, updates every 30 seconds with new requests
- When the user starts a new Copilot session (request arrives after >30min gap): the ticker resets
- When `active: false` (idle >30min): shows gray "idle" state
- Never shows "No sessions" if any session exists in the database

### Client-side: detecting new requests

The dashboard already polls every 30 seconds.
The live session panel compares `request_count` with the previous poll.
If `request_count` increased, the panel pulses briefly.

```js
if (current.request_count > this.lastCount) {
  this.pulse = true;
  setTimeout(() => this.pulse = false, 2000);
  this.lastCount = current.request_count;
}
```

### Implementation order

1. Add `CurrentSession` to store (50 lines)
2. Add `/api/session/current` handler (40 lines)
3. Dashboard live session panel (60 lines HTML + CSS)
4. Client-side pulse animation (30 lines JS)
5. Tests (40 lines)

---

## Implementation Priority

| Feature | Effort | Value | Order |
|---|---|---|---|
| Export | Small | High (data portability) | 1st |
| Period comparison | Medium | Highest (the question users ask) | 2nd |
| Live session | Small | Medium (session awareness) | 3rd |

## File Changes Summary

| File | Export | Compare | Live Session |
|---|---|---|---|
| `internal/store/store.go` | +30 | +60 | +50 |
| `internal/api/api.go` | +40 | +50 | +40 |
| `internal/cli/cli.go` | +60 | +80 | — |
| `internal/dashboard/index.html` | +20 | +80 | +60 |
| `internal/dashboard/app.js` | — | — | +30 |
| Test files | +40 | +60 | +40 |
| **Total** | **~190** | **~330** | **~220** |
