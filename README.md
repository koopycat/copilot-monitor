# Copilot Monitor

Local HTTP reverse proxy for GitHub Copilot model API calls in VSCode.
Captures usage metadata, token counts, and estimated GitHub Copilot AI-credit cost for every model interaction.

## Quickstart

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

Use Copilot normally. The proxy stores every model API request in SQLite.

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
