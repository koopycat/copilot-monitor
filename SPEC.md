# Copilot Monitor - Specification

## Overview

Copilot Monitor is a local HTTP reverse proxy for GitHub Copilot model API calls in VSCode.
VSCode is configured to send Copilot API traffic to `http://127.0.0.1:7733` via Copilot's override URL settings.
The proxy forwards those requests to the real GitHub Copilot upstreams, tees streaming responses in real time, and records model, token, latency, endpoint, and project metadata in SQLite.
The current CLI reports captured usage and estimated equivalent provider list-price cost.
Planned reports will add sessions and daily summaries.

## Terminology

This tool is not a generic HTTP forward proxy.
GitHub's official Copilot network settings documentation describes support for standard HTTP proxies configured in the editor or through `HTTPS_PROXY`, `https_proxy`, `HTTP_PROXY`, and `http_proxy`.
That mode can monitor, route, or terminate Copilot connections, but it cannot inspect HTTPS request and response bodies unless custom certificates or TLS inspection are used.
Because this project needs model names and token usage from JSON bodies, v1 uses Copilot's override URL settings instead of the documented generic proxy path.
This tool is a Copilot-specific local reverse proxy selected through `github.copilot.advanced.debug.overrideProxyUrl` and `github.copilot.advanced.debug.overrideCapiUrl`.

Reference: https://docs.github.com/en/copilot/concepts/network-settings

## Validated Behavior

The core proxy architecture has been validated against real VSCode Copilot traffic.
The validated client environment was VSCode `1.127.0` with Copilot Chat `0.55.0`.
The `debug.overrideProxyUrl` and `debug.overrideCapiUrl` settings successfully send Copilot traffic to `http://127.0.0.1:7733`.
VSCode sends `Authorization` headers to the local override URL, so the proxy does not need to read tokens from disk.
Streaming SSE responses include usage metadata for observed chat requests.
Anthropic-family models use the `/v1/messages` route and Anthropic-style usage fields (`input_tokens` and `output_tokens`).
Agent flows use a `/responses` WebSocket upgrade that must be tunneled.
VSCode also calls `/_ping` and `/models/session` during normal operation.

## Goals

- Capture Copilot model-generating API traffic that flows through `debug.overrideProxyUrl` and `debug.overrideCapiUrl`.
- Forward requests and responses with no user-observable behavior change in Copilot Chat or inline completions.
- Preserve streaming behavior by forwarding every SSE chunk to VSCode immediately while parsing a tee of the stream.
- Record enough metadata to answer which models were used, how many tokens were spent, which sessions produced them, and what the estimated equivalent provider list-price cost was.
- Stay a single static binary with local SQLite storage.
- Run on a single dev machine against the user's existing VSCode Copilot authentication.

## Non-Goals (v1)

- Acting as a generic `HTTP_PROXY` / `HTTPS_PROXY` forward proxy.
- Performing TLS interception or certificate installation.
- Reading GitHub or Copilot tokens from disk.
- Replacing or routing to alternative model providers.
- Capturing all Copilot telemetry, authentication, or token-refresh traffic.
- Inspecting WebSocket frame contents in v1. WebSocket agent traffic may be tunneled but not analyzed.
- A web UI.
- Centralized team aggregation.
- Modifying request bodies or response bodies.

## Architecture

```
VSCode Copilot extension
        |
        |  http://127.0.0.1:7733
        |  configured by debug.overrideProxyUrl and debug.overrideCapiUrl
        v
+----------------------------------------+
|  copilot-monitor (Go binary)           |
|                                        |
|  - HTTP server on 127.0.0.1:7733       |
|  - Path-based reverse proxy router     |
|  - Request metadata parser             |
|  - Real-time SSE tee parser            |
|  - SQLite writer                       |
+--------------------+--------------------+
                     |
                     |  HTTPS outbound to GitHub
                     v
   api.githubcopilot.com/...                  (chat, agents, models, embeddings)
   copilot-proxy.githubusercontent.com/...    (inline completions)
```

The local listener uses plain HTTP and is bound to `127.0.0.1` only.
No local TLS, no cert generation, no `NODE_EXTRA_CA_CERTS`, and no system trust changes are required.
Outbound connections from the proxy to GitHub use normal HTTPS through Go's stdlib `net/http` client.

## VSCode Configuration

VSCode is configured via `settings.json`:

```json
{
  "github.copilot.advanced": {
    "debug.overrideProxyUrl": "http://127.0.0.1:7733",
    "debug.overrideCapiUrl": "http://127.0.0.1:7733",
    "authProvider": "github"
  }
}
```

`debug.overrideProxyUrl` covers inline completions.
`debug.overrideCapiUrl` covers Copilot CAPI traffic such as chat, agents, models, and embeddings.
If the user removes these settings, VSCode connects to GitHub directly again.

## Routing

Routing is based on inbound request path, not the inbound `Host` header.
When VSCode sends requests to `http://127.0.0.1:7733`, the inbound `Host` header is expected to be `127.0.0.1:7733`.
The proxy rewrites the outbound URL and outbound `Host` header for the selected upstream.

| Inbound path | Upstream base URL | Capture behavior |
|---|---|---|
| `/chat/completions` | `https://api.githubcopilot.com` | Forward + capture model and usage |
| `/agents/*` | `https://api.githubcopilot.com` | Forward + capture model and usage when present |
| `/models` | `https://api.githubcopilot.com` | Forward only |
| `/models/session` | `https://api.githubcopilot.com` | Forward only |
| `/responses` | `https://api.githubcopilot.com` | WebSocket tunnel for agent responses |
| `/_ping` | local response | Return `200 OK` locally |
| `/embeddings` | `https://api.githubcopilot.com` | Forward metadata only if usage is present |
| `/v1/engines/*` | `https://copilot-proxy.githubusercontent.com` | Forward + capture inline completion usage when present |
| `/v1/completions` | `https://copilot-proxy.githubusercontent.com` | Forward + capture inline completion usage when present |
| `/v1/messages`, `/v1/messages/*` | `https://api.githubcopilot.com` | Forward + capture Anthropic-style message usage |
| Any other path | none | Return `502` and log `route=unknown` |

Unknown paths are intentionally rejected in v1 instead of silently forwarded.
This makes endpoint drift visible during development and prevents the proxy from hiding routing bugs.
The v1 proxy does not promise to capture Copilot telemetry or token refresh calls.
If such calls appear through the override URLs, they must be added explicitly or intentionally ignored.

## Authentication Handling

The proxy never reads GitHub tokens from disk.
The proxy never injects an `Authorization` header.
The proxy forwards incoming authentication headers from VSCode unchanged.
This avoids turning the local proxy into a token-bearing service that any local process could abuse.
Real VSCode traffic confirms `auth_present=true` for model API requests sent to the override URL.
Token loading is not part of v1.

## Streaming and Usage Requirements

Streaming responses must stay real time.
The proxy must not buffer a complete SSE response before sending it to VSCode.
For streaming endpoints, the implementation must:

- Read upstream bytes incrementally.
- Write each chunk to the downstream client immediately.
- Flush the downstream writer after each chunk when possible.
- Parse copied `data: ...` SSE lines opportunistically.
- Extract `usage` from the final chunk or final usage event when present.
- Persist the request record after the stream completes or fails.

OpenAI-compatible usage fields map as follows:

| Upstream field | Stored field |
|---|---|
| `prompt_tokens` | `prompt_tokens` |
| `completion_tokens` | `completion_tokens` |
| `total_tokens` | `total_tokens` |

Anthropic-compatible usage fields map as follows:

| Upstream field | Stored field |
|---|---|
| `input_tokens` | `prompt_tokens` |
| `output_tokens` | `completion_tokens` |
| derived sum | `total_tokens` when no explicit total exists |

A slow parser must never block response delivery beyond normal network backpressure.
If parsing fails, forwarding continues and the request is stored with missing token fields.

## CLI

Implemented commands:

```
copilot-monitor run [--addr 127.0.0.1:7733] [--db path] [--project name]
copilot-monitor configure-vscode [--addr 127.0.0.1:7733]
copilot-monitor stats [--db path] [--since 30d] [--project x] [--endpoint chat]
copilot-monitor cost [--db path] [--since 30d] [--project x] [--endpoint chat]
copilot-monitor today [--db path] [--project x] [--endpoint chat]
copilot-monitor sessions [--db path] [--since 30d] [--project x] [--limit 50]
copilot-monitor version
```

Planned commands:

```
copilot-monitor models
```

The `run` subcommand is the only command that opens a network listener.
All report commands are read-only against SQLite.

## Output Labels

Cost output must be labeled as estimated equivalent provider list-price cost.
It must not claim to be the user's actual GitHub Copilot bill.

Example `cost` output:

```
MODEL                      INPUT $   OUTPUT $    EST. LIST $
claude-sonnet-4            $0.0665    $0.0963       $0.1628
gpt-4o                     $0.1205    $0.1284       $0.2489
```

Example `stats` output:

```
MODEL                      REQUESTS   PROMPT_TOK   COMPL_TOK    TOTAL
gpt-4o                          142       48,210      12,840    61,050
claude-sonnet-4                  37       22,150       6,420    28,570
gemini-2.0-flash                 84       11,330       2,910    14,240
```

## Data Model

SQLite schema (`internal/store/schema.sql`):

```sql
CREATE TABLE requests (
  id                INTEGER PRIMARY KEY,
  ts                TEXT    NOT NULL,
  endpoint          TEXT    NOT NULL,
  method            TEXT    NOT NULL,
  path              TEXT    NOT NULL,
  upstream_host     TEXT    NOT NULL,
  model             TEXT,
  stream            INTEGER NOT NULL,
  status            INTEGER NOT NULL,
  error             TEXT,
  latency_ms        INTEGER NOT NULL,
  prompt_tokens     INTEGER,
  completion_tokens INTEGER,
  total_tokens      INTEGER,
  project           TEXT,
  session_id        INTEGER,
  request_hash      TEXT
);

CREATE TABLE sessions (
  id            INTEGER PRIMARY KEY,
  started_at    TEXT NOT NULL,
  ended_at      TEXT NOT NULL,
  project       TEXT,
  request_count INTEGER NOT NULL,
  token_count   INTEGER NOT NULL
);

CREATE TABLE bodies (
  request_id    INTEGER PRIMARY KEY REFERENCES requests(id),
  prompt        TEXT,
  completion    TEXT
);

CREATE INDEX idx_requests_ts       ON requests(ts);
CREATE INDEX idx_requests_model    ON requests(model);
CREATE INDEX idx_requests_project  ON requests(project);
CREATE INDEX idx_requests_session  ON requests(session_id);
CREATE INDEX idx_requests_endpoint ON requests(endpoint);
```

`bodies` is reserved for a future `--store-bodies` mode and is not populated by the current implementation.
`request_hash` is the SHA-256 hash of a canonicalized request body when the body can be read safely.

## Model Attribution

The proxy prefers the model name from the request body over the model name from the response body.
This is required because Copilot may use internal helper requests and may emit response model names such as `gpt-4o-mini-2024-07-18` for classifier or planning calls.
The response model is only used as a fallback when the request body has no parseable model.
Stats are per captured model API request, not per visible chat turn.
A single visible user prompt can produce multiple captured requests, including internal helper model calls.

## Sessions

A session is a maximal run of captured requests whose gaps between consecutive requests are all under 30 minutes.
The `sessions` command rebuilds sessions lazily from the captured request table before reading.
A request arriving 30 minutes or more after the previous captured request starts a new session.
Project tags are independent of session boundaries.
If a session contains multiple project tags, its session project is stored as `<mixed>`.

## Model Catalog

`internal/catalog/models.json` is embedded via `embed.FS`:

```json
{
  "models": {
    "gpt-4o":               {"provider":"openai",    "input_per_m": 2.50, "output_per_m": 10.00},
    "gpt-4o-2024-05-13":    {"provider":"openai",    "input_per_m": 2.50, "output_per_m": 10.00},
    "gpt-4.1":              {"provider":"openai",    "input_per_m": 2.00, "output_per_m":  8.00},
    "o1":                   {"provider":"openai",    "input_per_m": 15.00, "output_per_m": 60.00},
    "claude-sonnet-4":      {"provider":"anthropic", "input_per_m": 3.00, "output_per_m": 15.00},
    "claude-3-5-sonnet":    {"provider":"anthropic", "input_per_m": 3.00, "output_per_m": 15.00},
    "claude-opus-4":        {"provider":"anthropic", "input_per_m": 15.00, "output_per_m": 75.00},
    "gemini-2.0-flash":     {"provider":"google",    "input_per_m": 0.10, "output_per_m":  0.40},
    "gemini-2.5-pro":       {"provider":"google",    "input_per_m": 1.25, "output_per_m":  5.00}
  },
  "currency": "USD",
  "fallback_per_m": 5.00
}
```

Rates are per million tokens.
The fallback rate applies to unknown models and must be reported as a fallback in CLI output.
Editing the JSON and rebuilding the binary recomputes historical estimated list-price cost.

## Privacy

- Default behavior stores no request or response bodies.
- Body storage is planned but not implemented yet.
- Token counts, model names, latency, status, path, and timestamps are always stored for captured endpoints.
- The database lives only on the user's machine.
- Nothing is uploaded.
- Telemetry and token-refresh traffic are not a v1 capture target.

## Storage Locations

Following XDG Base Directory on Linux and the equivalent on macOS.

| What | Path |
|---|---|
| SQLite DB | `$XDG_DATA_HOME/copilot-monitor/store.db` |
| Logs | stderr only |

Defaults on Linux/macOS resolve `XDG_DATA_HOME` to `~/.local/share/copilot-monitor`.
On macOS the binary may use `~/Library/Application Support/copilot-monitor/` if XDG is unset.
No cert or key files are written by the proxy.

## Tech Stack

| Concern | Choice |
|---|---|
| Language | Go 1.25+ |
| DB driver | `modernc.org/sqlite` |
| CLI framework | stdlib `flag` with a small internal command dispatcher |
| Table output | stdlib `text/tabwriter` |
| HTTP server | stdlib `net/http`, plain HTTP on loopback |
| HTTP client | stdlib `net/http`, normal HTTPS upstream |
| JSON parsing | stdlib `encoding/json` |
| Tests | stdlib `testing` + `net/http/httptest` |

All dependencies must be available through `go get` with no system packages.

## Current File Layout

```
copilot_monitoring/
├── cmd/
│   ├── copilot-monitor/main.go
│   └── phase0/main.go
├── internal/
│   ├── cli/
│   │   ├── cli.go
│   │   └── cli_test.go
│   ├── proxy/
│   │   ├── capture.go
│   │   ├── preview.go
│   │   ├── router.go
│   │   ├── server.go
│   │   ├── sse.go
│   │   └── *_test.go
│   └── store/
│       ├── schema.sql
│       ├── store.go
│       └── store_test.go
├── go.mod
├── go.sum
├── PHASE0.md
├── PLAN.md
└── SPEC.md
```

## Implementation Status

Implemented:

1. `phase0` validation proxy.
2. `copilot-monitor run` with plain HTTP loopback listener.
3. VSCode settings printer.
4. Path router for observed VSCode Copilot routes.
5. Transparent forwarding with auth-header preservation.
6. Real-time SSE tee parsing.
7. WebSocket tunnel for `/responses`.
8. SQLite persistence of captured usage.
9. `stats` report grouped by model and endpoint.
10. Anthropic `/v1/messages` support.
11. Embedded public list-price model catalog.
12. `cost` report for estimated equivalent provider list-price cost.
13. 30-minute gap sessionization.
14. `today` and `sessions` reports.

Not yet implemented:

1. `models` command.
2. JSON report output.
3. Optional prompt/response body storage.
4. README quickstart.

## Future Work

- True generic HTTP proxy mode with optional TLS interception.
- Telemetry and token-refresh observability.
- Web dashboard embedded in the binary.
- CSV export.
- Budget alerts.
- Per-prompt timing breakdown.
- Project auto-detection from the nearest Git remote.
- Multi-machine aggregation.
