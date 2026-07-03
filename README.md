# Copilot Monitor

**Know exactly what GitHub Copilot is doing on your machine.**

A local HTTP reverse proxy that sits between VSCode and `api.githubcopilot.com`,
recording per-request metadata, token counts, latency, and an estimated
AI-credit cost. Everything is stored in a SQLite database on your machine.
No cloud. No telemetry. No prompts, completions, source code, or auth headers
are ever written to disk.

```sh
# run the proxy
./copilot-monitor run &

# start the dashboard API
./copilot-monitor serve
# → open http://127.0.0.1:7734
```

Project site: <https://koopycat.github.io/copilot-monitor/>

## Why

GitHub Copilot picks models automatically, charges AI credits invisibly, and
exposes usage summaries only on github.com with a 24-hour delay. You don't
know which model handled your last prompt, whether you got a cache hit, or
what that refactor session actually cost you.

Copilot Monitor gives you the raw numbers from your own machine:
**per-model token counts**, **latency**, **estimated cost**,
and **30-minute session groupings** — all in a local dashboard with a
period selector (today / yesterday / 7d / 30d / 90d / 365d) and a JSON API
for your own scripts.

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
./copilot-monitor version
```

In one terminal, start the dashboard API:

```sh
./copilot-monitor serve --addr 127.0.0.1:7734
```

In another terminal, verify it responds:

```sh
curl http://127.0.0.1:7734/api/health
```

Then start the proxy:

```sh
./copilot-monitor run --addr 127.0.0.1:7733
```

And verify the local ping endpoint:

```sh
curl http://127.0.0.1:7733/_ping
```

Stop long-running processes with `Ctrl+C` when finished.
Run `just all` before submitting changes.

## Development

Hot reload (server rebuilds on every `.go`, `.html`, or `.js` change):

```sh
just watch
```

Requires [air](https://github.com/air-verse/air) (`go install github.com/air-verse/air@latest`).

Live dashboard URL: `http://127.0.0.1:7734/`
The dashboard loads Petite-Vue from `unpkg` at runtime, so that page needs network access for the reactive runtime.

Print VSCode settings:

```sh
./copilot-monitor configure-vscode
```

Then paste the output into VSCode's `settings.json`.
Open it via **Cmd+Shift+P > Preferences: Open User Settings (JSON)**.

Start the proxy:

```sh
./copilot-monitor run
```

Reload VSCode:

```text
Cmd+Shift+P > Developer: Reload Window
```

Use Copilot normally. The proxy stores captured metadata and token counts in
SQLite for routes configured for persistence; it does not store prompts,
completions, source code, or auth material.

## Commands

```sh
./copilot-monitor stats --since all
./copilot-monitor cost --since all
./copilot-monitor today
./copilot-monitor sessions --since 7d --limit 20
```

JSON output for machine processing:

```sh
./copilot-monitor stats --since 7d --json
./copilot-monitor cost --since 7d --json
./copilot-monitor today --json
./copilot-monitor sessions --since 7d --json
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--addr` | `127.0.0.1:7733` | HTTP listen address, loopback only |
| `--db` | `~/.local/share/copilot-monitor/store.db` | SQLite database path |
| `--project` | none | optional project label for reporting |
| `--usage-debug-log` | none | optional JSONL path for pricing research |

`--usage-debug-log` is for local pricing research only. It should remain
metadata-only: no prompts, completions, source code, auth headers, cookies, or
API keys.

## What is stored

The database stores request metadata, endpoint, model name, token counts, latency, status, and project.
Token categories include input, cached input, cache write, and output tokens where available.
No prompts, completions, source code, or auth tokens are stored.

## What is not stored

Prompt text, completion text, source code, repository paths, auth headers, cookies, and API keys.

## Cost reporting

Cost output is an estimated equivalent GitHub Copilot AI-credit list-price estimate and is not your actual GitHub Copilot bill.
Pricing is sourced from GitHub's Copilot billing documentation [models and pricing](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing).
Agent metadata routes and code completions are excluded because they are not billed in AI credits.

## Data location

| What | Path |
|---|---|
| Database | `~/.local/share/copilot-monitor/store.db` |
| Debug log | `./usage-debug.jsonl` when `--usage-debug-log` is used |

## Privacy

All data stays on your machine. The proxy binds to `127.0.0.1` only.
Nothing is uploaded, ever. Anyone with shell access to your machine can read the database file.
