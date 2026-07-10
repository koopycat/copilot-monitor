# API & Dashboard

`copilot-monitor serve` starts the local HTTP API and embedded HTML dashboard on
a separate port from the proxy.

```text
Proxy:   copilot-monitor run    (port 7733, Copilot traffic)
API:     copilot-monitor serve  (port 7734, read-only)
```

## Endpoints

The reporting endpoints are read-only, but the session endpoints rebuild derived
session state before returning results. All support `?since=30d` and `?project=`
filters unless noted.

| Method | Path                   | Parameters                   | Returns                                                        |
| ------ | ---------------------- | ---------------------------- | -------------------------------------------------------------- | ------------------ |
| `GET`  | `/api/health`          | none                         | `{"ok":true}`                                                  |
| `GET`  | `/api/stats`           | `?since=&project=&endpoint=` | `[]ModelStats` with `avg_latency_ms` and compression fields    |
| `GET`  | `/api/cost`            | `?since=&project=&endpoint=` | `Total` with rows, aggregate, and `compression_removed_tokens` |
| `GET`  | `/api/today`           | `?project=&endpoint=`        | `[]ModelStats` since local midnight                            |
| `GET`  | `/api/sessions`        | `?since=&project=&limit=50`  | `[]SessionStats`                                               |
| `GET`  | `/api/stats/timeline`  | `?since=&granularity=day     | hour`                                                          | `[]TimelineBucket` |
| `GET`  | `/api/export`          | `?since=`                    | CSV with compression columns                                   |
| `GET`  | `/api/session/current` | none                         | Current session with per-model `compression_removed_tokens`    |
| `GET`  | `/`                    | none                         | HTML dashboard                                                 |

### Compression fields

When Headroom compression is configured, model stat responses include:

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

```
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
