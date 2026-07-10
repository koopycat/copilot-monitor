# Copilot Monitor

[![CI](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/koopycat/copilot-monitor)](https://github.com/koopycat/copilot-monitor/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
**Know exactly what your AI tools are costing you.**

A local HTTP reverse proxy that sits between your tools and LLM APIs, recording
per-request metadata, token counts, latency, and estimated cost. Everything is
stored in a SQLite database on your machine. No cloud. No telemetry. No prompts,
completions, source code, or auth headers are ever written to disk.

Works with GitHub Copilot out of the box, and any OpenAI-compatible or
Anthropic-compatible API via configurable routes (pi-agent, Claude Code, aider,
direct API calls, etc.).

```sh
# run the proxy
./bin/copilot-monitor run &

# start the dashboard API
./bin/copilot-monitor serve
# → open http://127.0.0.1:7734
```

Project site: <https://koopycat.github.io/copilot-monitor/>

## Download

Prebuilt binaries for the latest release. The links below always resolve to
whatever is currently the newest tag, so the table stays useful after every
release.

| OS      | Architecture  | Binary                                                                                                                                          |
| ------- | ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Linux   | x86_64        | [copilot-monitor-linux-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-amd64.tar.gz)   |
| Linux   | ARM64         | [copilot-monitor-linux-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-arm64.tar.gz)   |
| macOS   | Intel         | [copilot-monitor-darwin-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-amd64.tar.gz) |
| macOS   | Apple Silicon | [copilot-monitor-darwin-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-arm64.tar.gz) |
| Windows | x86_64        | [copilot-monitor-windows-amd64.zip](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-windows-amd64.zip)     |

Extract and run:

```sh
tar -xzf copilot-monitor-linux-amd64.tar.gz
./copilot-monitor run &
./copilot-monitor serve
# → open http://127.0.0.1:7734
```

Verify the binary matches the release on the
[releases page](https://github.com/koopycat/copilot-monitor/releases) if you
care about supply-chain integrity. For a reproducible build from source, see
[Quickstart](#quickstart) below.

## Why

GitHub Copilot picks models automatically, charges AI credits invisibly, and
exposes usage summaries only on github.com with a 24-hour delay. You don't know
which model handled your last prompt, whether you got a cache hit, or what that
refactor session actually cost you.

Copilot Monitor gives you the raw numbers from your own machine: per-model token
counts, latency, estimated cost, and 30-minute session groupings, all in a local
dashboard with a period selector (today / yesterday / 7d / 30d / 90d / 365d) and
a JSON API for your own scripts.

## What it looks like

```text
est. AI-credit cost, 30d        Live Session
$8.42   projected this month    ● active  $0.13   24 reqs   44,602 tok
974 requests, 30d

USAGE  [Day | Hour]  [Tokens | Requests]
  ▆▅▆▇█▆▇█▅▆▇█▇█▆▇▆▇█▆▇▇█▆▇▆▇
  claude-3.5-sonnet  gpt-4.1  o3  gpt-4.1-mini

MODELS
  claude-3.5-sonnet  113 req   286k tok   2.5s   $2.95
  gpt-4.1            209 req   372k tok   2.4s   $2.85
  o3                  102 req   320k tok   2.7s   $2.35
  gpt-4.1-mini        110 req   147k tok   2.6s   $0.27
```

## Quickstart

Use the devenv shell for the expected Go toolchain. The project baseline is Go
1.26, and CI/release builds use patched Go 1.26.4.

Enter the devenv shell:

```sh
devenv shell
```

Build:

```sh
just build
```

Run all checks:

```sh
just all
```

## Contributor Smoke Test

For a first local verification after checkout or before a small change:

```sh
devenv shell
just test
just build
./bin/copilot-monitor version
```

In one terminal, start the dashboard API:

```sh
./bin/copilot-monitor serve --addr 127.0.0.1:7734
```

In another terminal, verify it responds:

```sh
curl http://127.0.0.1:7734/api/health
```

Then start the proxy:

```sh
./bin/copilot-monitor run --addr 127.0.0.1:7733
```

And verify the local ping endpoint:

```sh
curl http://127.0.0.1:7733/copilot/_ping
```

Stop long-running processes with `Ctrl+C` when finished. Run `just all` before
submitting changes.

## Using with Pi Agent and Other LLM Tools

Copilot Monitor can proxy any OpenAI-compatible or Anthropic-compatible API by
configuring additional routes via `--routes-config`.

### Pi Agent (KiloCode gateway)

Configure pi to route its API calls through the proxy:

**1. Create a routes config** (`routes.json`):

```json
{
  "routes": [
    {
      "path": "/chat/completions",
      "upstream_host": "api.kilo.ai",
      "upstream_path_prefix": "/api/gateway",
      "capture": "usage"
    },
    {
      "path": "/models",
      "upstream_host": "api.kilo.ai",
      "upstream_path_prefix": "/api/gateway",
      "capture": "none"
    }
  ]
}
```

**2. Start the proxy** with the routes config:

```sh
./bin/copilot-monitor run --routes-config routes.json
```

**3. Start pi** with its base URL pointing at the proxy:

```sh
KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi
```

Pi will now send all API requests through the proxy. Token counts, model names,
latency, and estimated cost are captured and visible in the dashboard and CLI
reports, just like Copilot traffic.

### Other tools (OpenAI, Anthropic, Ollama, etc.)

The same pattern works for any tool that speaks OpenAI-compatible or
Anthropic-compatible HTTP. Point your tool at the proxy using the appropriate
path-prefix (e.g., `http://127.0.0.1:7733/copilot` for VSCode Copilot,
`http://127.0.0.1:7733/openai/v1` for OpenAI-compatible tools, or
`http://127.0.0.1:7733/kilo` for Kilo) and add the corresponding routes:

```json
{
  "routes": [
    {
      "path": "/v1/chat/completions",
      "upstream_host": "api.openai.com",
      "capture": "usage"
    },
    {
      "path": "/v1/messages",
      "upstream_host": "api.anthropic.com",
      "capture": "usage",
      "prefix_match": true
    },
    {
      "path": "/v1/chat/completions",
      "upstream_host": "localhost:11434",
      "capture": "usage"
    }
  ]
}
```

Each route maps an incoming request path to an upstream host. Set `capture` to
`"usage"` (track tokens), `"metadata"` (track requests without token counts), or
`"none"` (forward without recording). Use `prefix_match: true` for routes that
match a path and all sub-paths (e.g., Anthropic's `/v1/messages` and
`/v1/messages/count_tokens`).

### Optional local request compression

Run a local [Headroom](https://github.com/headroomai/headroom) process and point
the proxy at its compression endpoint:

```sh
./bin/copilot-monitor run \
  --routes-config routes.json \
  --headroom-url http://127.0.0.1:8787/v1/compress
```

Compression applies automatically to supported OpenAI-compatible chat requests
after routing and model policy checks. The default is fail-open: if Headroom is
unavailable, the original request is forwarded. Add `--headroom-required` to
return HTTP 502 instead. `--headroom-compress-user-messages` and
`--headroom-target-ratio 0.5` expose the corresponding Headroom policy controls.
The compression endpoint must be loopback HTTP; no separate privacy consent step
is used for this personal, single-user tool.

**Privacy**: Headroom is a separate local process and may retain original
content in its CCR (Compress-Cache-Retrieve) store according to its own
configuration. Run Headroom with `--stateless` to disable all filesystem writes.
Copilot Monitor's own database never stores request or response bodies.

### Running proxy and dashboard together

```sh
# Terminal 1: proxy captures API traffic
./bin/copilot-monitor run --routes-config routes.json

# Terminal 2: dashboard shows captured data
./bin/copilot-monitor serve

# Terminal 3 (or herdr pane): your tool pointing at the proxy
KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi
```

### Policy — Model Allow/Block

The proxy can enforce a global model policy to control which AI models your
tools can use.

**Modes:**

- `allow_all` (default) — all models pass through
- `blocklist` — listed models are blocked (use `gpt-*` for prefix matching)
- `allowlist` — only listed models are allowed

**Via the dashboard:** Open the dashboard at http://127.0.0.1:7734 and look for
the "Security Policy" panel at the bottom. Click "Edit" to switch modes and add
model patterns.

**Via the API:**

```bash
# Block gpt-4o and all gpt-4.1 variants
curl -X PUT http://127.0.0.1:7734/api/policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"blocklist","models":["gpt-4o","gpt-4.1-*"]}'

# View current policy
curl http://127.0.0.1:7734/api/policy

# Reset to allow all
curl -X PUT http://127.0.0.1:7734/api/policy \
  -H "Content-Type: application/json" \
  -d '{"mode":"allow_all","models":[]}'
```

Blocked requests return HTTP 403 and are logged to the dashboard.

## Development

Hot reload (server rebuilds on every `.go`, `.html`, or `.js` change):

```sh
just watch
```

Requires [air](https://github.com/air-verse/air)
(`go install github.com/air-verse/air@latest`).

Live dashboard URL: `http://127.0.0.1:7734/` The dashboard is a Svelte 5 app
built with Vite and embedded in the Go binary. No runtime network access is
needed for the dashboard itself — all assets are served locally.

Start the proxy:

```sh
./bin/copilot-monitor run
```

While the proxy runs, a live session tail refreshes every 2 seconds in your
terminal: status, duration, request count, tokens, and estimated cost for the
current session. When the tail is active, the per-request log is suppressed so
the two streams do not interleave and corrupt the live display. Pass `--no-live`
to disable the tail and keep the full request log (also useful when stderr is
redirected to a log file).

Use Copilot normally. The proxy stores captured metadata and token counts in
SQLite for routes configured for persistence; it does not store prompts,
completions, source code, or auth material.

## Commands

```sh
./bin/copilot-monitor stats --since all
./bin/copilot-monitor cost --since all
./bin/copilot-monitor today
./bin/copilot-monitor live
./bin/copilot-monitor sessions --since 7d --limit 20
```

JSON output for machine processing:

```sh
./bin/copilot-monitor stats --since 7d --json
./bin/copilot-monitor cost --since 7d --json
./bin/copilot-monitor today --json
./bin/copilot-monitor live --json
./bin/copilot-monitor sessions --since 7d --json
```

`live` prints the current active session: status, project, duration, request
count, total tokens, and a per-model overview with cache hit rate and cost. This
is the same data the dashboard's "Live Session" panel shows, so you can check
what is happening right now without opening a browser.

Add `--watch` to keep it refreshing on screen (like `watch` on Unix):

```sh
./bin/copilot-monitor live --watch
```

Press `Ctrl+C` to stop.

## Flags

| Flag                                | Default                                   | Description                                      |
| ----------------------------------- | ----------------------------------------- | ------------------------------------------------ |
| `--addr`                            | `127.0.0.1:7733`                          | HTTP listen address, loopback only               |
| `--db`                              | `~/.local/share/copilot-monitor/store.db` | SQLite database path                             |
| `--project`                         | none                                      | optional project label for reporting             |
| `--usage-debug-log`                 | none                                      | optional JSONL path for pricing research         |
| `--headroom-url`                    | none                                      | loopback Headroom `/v1/compress` endpoint        |
| `--headroom-timeout`                | `30s`                                     | Headroom compression request timeout             |
| `--headroom-required`               | false                                     | fail requests instead of forwarding uncompressed |
| `--headroom-compress-user-messages` | false                                     | allow Headroom to transform user messages        |
| `--headroom-target-ratio`           | 0                                         | optional Headroom target ratio (0 < ratio <= 1)  |

`--usage-debug-log` is for local pricing research only. It should remain
metadata-only: no prompts, completions, source code, auth headers, cookies, or
API keys.

## What is stored

The database stores request metadata, endpoint, model name, token counts,
latency, status, and project. Token categories include input, cached input,
cache write, and output tokens where available. When Headroom compression is
configured, estimated compression metrics (status, original tokens, final
tokens, compression latency) are persisted as nullable columns. No prompts,
completions, source code, or auth tokens are stored.

## What is not stored

Prompt text, completion text, source code, repository paths, auth headers,
cookies, and API keys.

## Cost reporting

Cost output is an estimated equivalent GitHub Copilot AI-credit list-price
estimate and is not your actual GitHub Copilot bill. Pricing is sourced from
GitHub's Copilot billing documentation
[models and pricing](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing).
Agent metadata routes and code completions are excluded because they are not
billed in AI credits.

## Data location

| What      | Path                                                   |
| --------- | ------------------------------------------------------ |
| Database  | `~/.local/share/copilot-monitor/store.db`              |
| Debug log | `./usage-debug.jsonl` when `--usage-debug-log` is used |

## Privacy

All data stays on your machine. The proxy binds to `127.0.0.1` only. Nothing is
uploaded, ever. Anyone with shell access to your machine can read the database
file.
