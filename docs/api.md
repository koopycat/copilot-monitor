# API & Dashboard

`copilot-monitor serve` starts a read-only HTTP API and embedded HTML dashboard on a separate port from the proxy.

```text
Proxy:   copilot-monitor run    (port 7733, Copilot traffic)
API:     copilot-monitor serve  (port 7734, read-only)
```

## Endpoints

All endpoints are read-only. All support `?since=30d` and `?project=` filters unless noted.

| Method | Path | Parameters | Returns |
|---|---|---|---|
| `GET` | `/api/health` | none | `{"ok":true}` |
| `GET` | `/api/stats` | `?since=&project=&endpoint=` | `[]ModelStats` with `avg_latency_ms` |
| `GET` | `/api/cost` | `?since=&project=&endpoint=` | `Total` with rows and aggregate |
| `GET` | `/api/today` | `?project=&endpoint=` | `[]ModelStats` since local midnight |
| `GET` | `/api/sessions` | `?since=&project=&limit=50` | `[]SessionStats` |
| `GET` | `/api/stats/timeline` | `?since=&granularity=day|hour` | `[]TimelineBucket` |
| `GET` | `/api/export` | `?since=` | CSV dump of raw requests |
| `GET` | `/api/compare` | `?a=YYYY-MM&b=YYYY-MM` or `?periods=2&bucket=month` | Two-period comparison with totals |
| `GET` | `/` | none | HTML dashboard |

## Dashboard

Embedded single-page HTML dashboard served by the API server.
Petite-Vue for reactivity, canvas chart for timeline.
No build step. Petite-Vue is loaded by an import map from `unpkg`, so the
dashboard currently has a CDN dependency for the reactive runtime.

Features:

- Cost summary with projected monthly estimate
- Average latency metric
- Stacked bar chart with day/hour toggle
- Two-period comparison panel
- Per-model tables with token counts and latency
- Cost breakdown with fallback and not-billed tags
- Recent sessions table
- Manual refresh button
- Auto-refresh every 30 seconds
- CSV export link

## Files

```
internal/
├── api/api.go           # HTTP handler, all endpoints
├── dashboard/
│   ├── dashboard.go     # embed.FS and file server
│   ├── index.html       # HTML shell, CSS, DOM bindings
│   ├── app.js           # Petite-Vue app, state, fetch
│   └── chart.js         # Canvas stacked bar chart
```
