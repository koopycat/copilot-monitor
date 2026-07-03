# Copilot Monitor

Local HTTP reverse proxy for GitHub Copilot model API calls in VSCode.
Captures usage metadata, token counts, and estimated GitHub Copilot AI-credit cost for every model interaction.

## Quickstart

Use the devenv shell for the expected Go toolchain. `devenv.nix` currently provides Go 1.26; `go.mod` declares the module language version as Go 1.25.0.

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
