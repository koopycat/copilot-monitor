# Visualization API Plan

## Decision: `copilot-monitor serve`

Add a `serve` subcommand that starts a read-only HTTP API on a separate port from the proxy.
The proxy port (7733) stays dedicated to Copilot traffic forwarding.
The API port (7734) serves JSON data for external dashboards and visualizations.

## Architecture

```
copilot-monitor run --db store.db       (port 7733, Copilot proxy)
copilot-monitor serve --db store.db     (port 7734, read-only API)

VSCode --> :7733 (proxy) --> GitHub Copilot
                          |
                          v
                      store.db (SQLite)

Dashboard --> :7734/api/stats
Dashboard --> :7734/api/cost
Dashboard --> :7734/api/sessions
Dashboard --> :7734/api/today
```

## Endpoints

All endpoints return JSON. All are read-only. All accept the same filter parameters as their CLI counterparts.

| Method | Path | Parameters | Returns |
|---|---|---|---|
| `GET` | `/api/health` | none | `{"ok":true,"version":"...","db":"..."}` |
| `GET` | `/api/stats` | `?since=30d&project=&endpoint=` | `[]ModelStats` |
| `GET` | `/api/cost` | `?since=30d&project=&endpoint=` | `Total` (rows + aggregate) |
| `GET` | `/api/today` | `?project=&endpoint=` | `[]ModelStats` |
| `GET` | `/api/sessions` | `?since=30d&project=&limit=50` | `[]SessionStats` |
| `GET` | `/` | none | HTML dashboard (embedded via `embed.FS`) |

## HTML Dashboard (embedded, v1)

A single static HTML page embedded in the binary.
No JavaScript framework, no build step.
Pure HTML + vanilla JS + inline CSS.

Features:

- Auto-refresh every 30 seconds.
- Three panels: cost summary, model breakdown, recent sessions.
- Fetches data from the API endpoints the server already exposes.
- Uses a simple table and maybe a small bar chart (CSS-only or a tiny inline chart library).

The HTML file lives at `internal/dashboard/index.html` and is embedded via `embed.FS`.

## Implementation steps

1. Add `internal/api` package with an HTTP handler.
2. Wire it to `copilot-monitor serve --addr 127.0.0.1:7734 --db path`.
3. Reuse existing `store.Stats`, `store.Sessions`, `cost.Calculate` functions.
4. Add `GET /api/health`.
5. Add the four data endpoints.
6. Add `internal/dashboard/index.html` with embedded FS.
7. Serve it at `GET /`.
8. Update `SPEC.md`, `PLAN.md`, and `README.md`.

## Design constraints

- The `serve` command does not modify the database.
- Session rebuild runs lazily when `/api/sessions` is called.
- CORS headers are set to `*` for local dashboard development.
- The dashboard is self-contained; no external CDN dependencies.
- No authentication in v1 (loopback only).
