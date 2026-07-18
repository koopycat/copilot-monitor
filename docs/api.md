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

| Method | Path                              | Parameters                                   | Returns                                                               |
| ------ | --------------------------------- | -------------------------------------------- | --------------------------------------------------------------------- |
| `GET`  | `/api/health`                     | none                                         | Health, including retention status                                    |
| `GET`  | `/api/stats`                      | `?since=&project=&endpoint=`                 | `[]ModelStats` with latency and headroom-proxied flag, inference only |
| `GET`  | `/api/cost`                       | `?since=&project=&endpoint=`                 | Published token-rate estimate, inference only                         |
| `GET`  | `/api/today`                      | `?project=&endpoint=`                        | `[]ModelStats` since local midnight, inference only                   |
| `GET`  | `/api/sessions`                   | `?since=&project=&limit=&cursor=&cursor_id=` | `[]SessionStats`, newest first                                        |
| `GET`  | `/api/sessions/count`             | `?since=&until=&project=`                    | `{"count": N}` for the matching session filter                        |
| `GET`  | `/api/sessions/distinct-projects` | none                                         | Sorted `[]string` project names                                       |
| `GET`  | `/api/anomalies`                  | `?category=&severity=`                       | Up to 50 recent anomalies                                             |
| `GET`  | `/api/stats/timeline`             | `?since=&granularity=day\|hour`              | `[]TimelineBucket`, inference only                                    |
| `GET`  | `/api/export`                     | `?since=`                                    | CSV with `endpoint_kind` and `headroom_proxied` columns               |
| `GET`  | `/api/session/current`            | none                                         | Current session with per-model stats and headroom-proxied flag        |
| `GET`  | `/`                               | none                                         | HTML dashboard                                                        |

`/api/health` includes `retention_days`, `last_prune_at` (or `null` before the
first run), and `pruned_count` from the latest retention pass. For session
pagination, pass the final row's `started_at` and `id` as `cursor` and
`cursor_id` to retrieve the next older page.

### Inference-only usage views

`/api/stats`, `/api/cost`, `/api/today`, and `/api/stats/timeline` include only
model-generation traffic (`endpoint_kind = inference`). Control-plane traffic
such as `GET /models` and `GET /agents` is still captured but is not counted in
usage totals, cost estimates, or timeline buckets. Use `/api/export` for a
full-fidelity list that includes every `endpoint_kind`.

### Cost estimate semantics

`/api/cost` returns a local published token-rate estimate, not account billing
or invoice reconciliation. Its `estimate` object includes `currency`,
`rate_source` when available, `basis` (`published_token_rates`), and
`billing_scope` (`not_invoice_reconciliation`).

### Headroom-proxied flag

When an optional Headroom compression proxy runs in front of Copilot Monitor,
the proxy detects it via RemoteAddr and sets a `headroom_proxied` flag on the
captured request. This flag is available in:

- Model stat responses as `headroom_proxied` (boolean per-request, or count in
  aggregates)
- Export CSV rows as a `headroom_proxied` column
- Session model stats as `headroom_proxied` (boolean indicating at least one
  request in the group arrived via Headroom)

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
