# API & Dashboard

`copilot-monitor serve` starts the local HTTP API and embedded HTML dashboard on
a separate port from the proxy.

```text
Proxy:   copilot-monitor run    (port 7733, Copilot traffic)
API:     copilot-monitor serve  (port 7734, read-only)
```

## Endpoints

The reporting endpoints are read-only. Session state is maintained when requests
are captured, so reporting endpoints do not rebuild it. All support `?since=30d`
and `?project=` filters unless noted. Server failures use the structured
response `{"error":"internal server error"}`; details are logged only on the
server.

| Method | Path                              | Parameters                                   | Returns                                            |
| ------ | --------------------------------- | -------------------------------------------- | -------------------------------------------------- |
| `GET`  | `/api/health`                     | none                                         | Health, including retention status                 |
| `GET`  | `/api/stats`                      | `?since=&project=&endpoint=`                 | `[]ModelStats` with latency and compression fields |
| `GET`  | `/api/cost`                       | `?since=&project=&endpoint=`                 | Total cost and rows                                |
| `GET`  | `/api/today`                      | `?project=&endpoint=`                        | `[]ModelStats` since local midnight                |
| `GET`  | `/api/sessions`                   | `?since=&project=&limit=&cursor=&cursor_id=` | `[]SessionStats`, newest first                     |
| `GET`  | `/api/sessions/count`             | `?since=&until=&project=`                    | `{"count": N}` for the matching session filter     |
| `GET`  | `/api/sessions/distinct-projects` | none                                         | Sorted `[]string` project names                    |
| `GET`  | `/api/anomalies`                  | `?category=&severity=`                       | Up to 50 recent anomalies                          |
| `GET`  | `/api/stats/timeline`             | `?since=&granularity=day\|hour`              | `[]TimelineBucket`                                 |
| `GET`  | `/api/export`                     | `?since=`                                    | CSV with compression columns                       |
| `GET`  | `/api/session/current`            | none                                         | Current session with per-model compression metrics |
| `GET`  | `/`                               | none                                         | HTML dashboard                                     |

`/api/health` includes `retention_days`, `last_prune_at` (or `null` before the
first run), and `pruned_count` from the latest retention pass. For session
pagination, pass the final row's `started_at` and `id` as `cursor` and
`cursor_id` to retrieve the next older page.

### Compression fields

When compression is configured on a route, model stat responses include:

- `compressed_requests` -- requests with compression applied
- `compression_original_tokens` -- estimated original input tokens
- `compression_final_tokens` -- estimated compressed input tokens
- `compression_removed_tokens` -- estimated tokens removed
- `avg_compression_ratio` -- average compression ratio

Export rows include `compression_status`, `compression_original_tokens`,
`compression_final_tokens`, and `compression_latency_ms`. Cost rows and
current-session model rows include `compressed_requests` and
`compression_removed_tokens`.

## Dashboard

Embedded single-page Svelte 5 dashboard served by the API server. Built with
Vite and embedded in the Go binary. No runtime CDN dependencies.

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

```text
internal/
├── api/api.go           # HTTP handler, all endpoints
dashboard/
├── src/
│   ├── lib/             # types, api client, formatters
│   ├── stores/          # Svelte 5 reactive dashboard store
│   └── components/      # ModelsTable, SessionsTable, LiveSessionCard, etc.
├── index.html           # Svelte entry point
└── vite.config.ts       # Vite build configuration
```
