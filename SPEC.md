# Copilot Monitor

Local HTTP reverse proxy for GitHub Copilot model API calls in VSCode.
Captures usage metadata, token counts, and estimated GitHub Copilot AI-credit cost.

## Documentation

| Document | Content |
|---|---|
| [`SPEC.md`](SPEC.md) | Core proxy architecture, routing, persistence, CLI (this file) |
| [`docs/api.md`](docs/api.md) | Read-only HTTP API and embedded dashboard |
| [`docs/statistics.md`](docs/statistics.md) | Planned statistics, visualizations, and timeline queries |

| [`docs/advanced-analytics.md`](docs/advanced-analytics.md) | Planned features: period comparison, live session view |

## Platform

| Concern | Tool |
|---|---|
| Language | Go 1.26+ (managed via [devenv](https://devenv.sh/)) |
| Build | `just build` |
| Test | `just test` |
| Static analysis | `go vet`, `staticcheck`, `govulncheck` (`just vet`) |
| All checks | `just all` |

## CLI

All commands support `--db path` to override the default SQLite database location (`~/.local/share/copilot-monitor/store.db`).

### Proxy

```sh
copilot-monitor run [--addr 127.0.0.1:7733] [--project name] [--usage-debug-log path]
```

Starts the local HTTP reverse proxy.
Forwards Copilot model API calls from VSCode to GitHub, capturing usage metadata.

### API & Dashboard

```sh
copilot-monitor serve [--addr 127.0.0.1:7734]
```

Starts a read-only HTTP API and embedded HTML dashboard.
Full endpoint reference: [`docs/api.md`](docs/api.md).

### Reports

```sh
copilot-monitor stats [--since 30d] [--project x] [--endpoint chat] [--json]
copilot-monitor cost  [--since 30d] [--project x] [--endpoint chat] [--json]
copilot-monitor today [--project x] [--endpoint chat] [--json]
copilot-monitor sessions [--since 30d] [--project x] [--limit 50] [--json]
copilot-monitor compare [--a 2026-06] [--b 2026-07] [--db path]
```

### Setup

```sh
copilot-monitor configure-vscode
copilot-monitor export [--since 30d] [--db path]
copilot-monitor version
```

## Data Model

SQLite schema with three tables:

- `requests` - every captured model API call with model, token counts, latency, status, endpoint, project, and session assignment
- `sessions` - 30-minute inactivity-gap session groups with start, end, project, request count, and token count
- `bodies` - reserved for optional body storage (not populated by default)

Token categories stored: prompt tokens, cached input tokens, cache write tokens, completion tokens, and total tokens.
OpenAI `prompt_tokens_details.cached_tokens` and Anthropic `cache_read_input_tokens` / `cache_creation_input_tokens` fields are parsed.

## Routing

The proxy routes inbound paths to the appropriate GitHub Copilot upstream.
Unknown paths return `502` and log `route=unknown`.

| Inbound path | Upstream | Capture |
|---|---|---|
| `/chat/completions` | `api.githubcopilot.com` | Yes |
| `/agents/*` | `api.githubcopilot.com` | Yes (usage only) |
| `/models`, `/models/session` | `api.githubcopilot.com` | No |
| `/responses` | `api.githubcopilot.com` | WebSocket tunnel |
| `/_ping` | local response | No |
| `/v1/engines/*`, `/v1/completions` | `copilot-proxy.githubusercontent.com` | Yes |
| `/v1/messages`, `/v1/messages/*` | `api.githubcopilot.com` | Yes (Anthropic) |

## Pricing

Prices are GitHub Copilot AI-credit list prices sourced from [GitHub's billing documentation](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing).
The embedded catalog (`internal/catalog/models.json`) includes exact prices for known models and provider-level fallbacks for unknown models.
Cost output is labeled as estimated equivalent list-price cost.
Code completions are shown with zero AI-credit cost because GitHub docs state they are not billed in AI credits.

## Privacy

Default behavior stores no request or response bodies.
Token counts, model names, latency, status, and timestamps are always stored for captured endpoints.
The database lives only on the user's machine.
Nothing is uploaded.
The proxy binds to `127.0.0.1` only.
Anyone with shell access to the machine can read the database file.

## File Layout

```
copilot_monitoring/
├── cmd/
│   ├── copilot-monitor/main.go
│   └── phase0/main.go
├── internal/
│   ├── api/                    # Read-only HTTP API handler
│   ├── catalog/                # Embedded model price catalog
│   ├── cli/                    # Command dispatcher
│   ├── cost/                   # Cost calculator
│   ├── dashboard/              # Embedded HTML dashboard
│   ├── log/                    # Colored structured logger
│   ├── proxy/                  # HTTP reverse proxy (core)
│   └── store/                  # SQLite persistence
├── docs/
│   ├── api.md
│   └── statistics.md
├── devenv.nix
├── devenv.lock
├── justfile
├── go.mod / go.sum
├── SPEC.md / PLAN.md / README.md
└── PHASE0.md
```
