# Copilot Monitor

[![CI](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/koopycat/copilot-monitor)](https://github.com/koopycat/copilot-monitor/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**The local flight recorder for your AI coding traffic.**

Copilot Monitor is a local HTTP proxy and SQLite-backed dashboard for GitHub
Copilot and compatible AI clients. It records model, token and cache usage, HTTP
status, end-to-end latency, and estimated model-rate cost on your machine. By
default, the proxy listens only on `127.0.0.1`; the normal database capture does
not store prompts, completions, source code, or auth headers.

Each `run` process forwards to one explicit `--upstream` host. This is a
developer-side traffic recorder, not a multi-provider gateway or shared
observability platform: it does not manage provider routing, virtual keys,
quotas, or invoice reconciliation. It complements those systems by making the
traffic from your own tools inspectable without an SDK, collector, or monitoring
account.

<!-- TODO: add dashboard screenshot -->

> See the [project website](https://koopycat.github.io/copilot-monitor/) for
> dashboard screenshots.

## Install

Download an archive for your platform, extract it, and put `copilot-monitor` on
your `PATH`.

| Platform       | Download                                                                                                                                        |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| macOS ARM64    | [copilot-monitor-darwin-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-arm64.tar.gz) |
| macOS Intel    | [copilot-monitor-darwin-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-amd64.tar.gz) |
| Linux x86_64   | [copilot-monitor-linux-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-amd64.tar.gz)   |
| Linux ARM64    | [copilot-monitor-linux-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-arm64.tar.gz)   |
| Windows x86_64 | [copilot-monitor-windows-amd64.zip](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-windows-amd64.zip)     |

Homebrew:

```sh
brew tap koopycat/copilot-monitor
brew trust koopycat/copilot-monitor  # signs the tap for Homebrew verification
brew install copilot-monitor
```

Build from source. `just build` writes the binary to `./bin/copilot-monitor`.

```sh
devenv shell
just build
./bin/copilot-monitor version
```

Verify an installed binary before continuing:

```sh
copilot-monitor version
```

## Client setup and compatibility

Your client must support a base-URL or upstream override. The proxy forwards all
non-health requests unchanged to the one configured `--upstream` host.

| Client                   | Setup                                         | Coverage                                                                    |
| ------------------------ | --------------------------------------------- | --------------------------------------------------------------------------- |
| VS Code + GitHub Copilot | One-time advanced setting and a window reload | HTTP and Copilot `/responses` WebSocket usage capture                       |
| pi + Kilo gateway        | Set `KILO_GATEWAY_BASE_URL` to the proxy      | First-capture path documented below                                         |
| Other API clients        | Point a custom base URL at the proxy          | Best effort; paths and headers are forwarded unchanged to one upstream host |

Model policy applies to HTTP requests and complete Copilot WebSocket text
messages that explicitly name a model. As with HTTP requests that omit a model,
invalid or over-limit WebSocket messages fail open rather than interrupting
traffic the proxy cannot identify safely.

VS Code is not auto-detected. Open **Preferences: Open User Settings (JSON)**
and merge this setting, then run **Developer: Reload Window**:

```json
{
  "github.copilot.advanced": {
    "debug.overrideCapiUrl": "http://127.0.0.1:7733"
  }
}
```

Then start the proxy:

```sh
copilot-monitor run --upstream api.githubcopilot.com
```

If the dashboard is unavailable or no capture appears after a request, run:

```sh
copilot-monitor doctor --upstream api.githubcopilot.com
```

It checks the local proxy, dashboard, database path, and optional upstream TCP
reachability without creating or migrating the database. It cannot inspect your
editor's private settings, so keep the VS Code override and reload step above.

## First capture with pi/Kilo

This path requires a separately installed and authenticated `pi` client.

In terminal 1, start the proxy and dashboard, forwarding to your upstream:

```sh
copilot-monitor run --dashboard --upstream api.githubcopilot.com
```

In terminal 2, verify both services, make one request through the proxy, then
inspect the capture:

```sh
curl http://127.0.0.1:7733/_health
curl http://127.0.0.1:7734/api/health
KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733 pi -p 'Reply OK'
copilot-monitor stats --since 1h
copilot-monitor doctor --upstream api.githubcopilot.com
```

## Commands

| Command      | Purpose                                                                      |
| ------------ | ---------------------------------------------------------------------------- |
| `run`        | Start the local proxy; `--dashboard` also starts the dashboard on port 7734. |
| `doctor`     | Check local proxy, dashboard, database, and optional upstream setup.         |
| `serve`      | Start the dashboard and read-only API from an existing database.             |
| `stats`      | Show captured usage by model and endpoint.                                   |
| `cost`       | Show estimated model-rate cost; not a billing invoice.                       |
| `today`      | Show usage since local midnight.                                             |
| `sessions`   | List sessions derived from a 30-minute inactivity gap.                       |
| `live`       | Show the current session; use `--watch` to refresh.                          |
| `export`     | Export captured request metadata as CSV.                                     |
| `inspect`    | Show detected proxy anomalies.                                               |
| `completion` | Generate zsh shell completion scripts.                                       |
| `version`    | Print the installed version.                                                 |
| `help`       | Show command usage.                                                          |

Reporting and inspection commands, including `stats`, `cost`, `today`,
`sessions`, `live`, and `inspect`, support `--json`. `export` writes CSV only.
`doctor` also supports `--json`. Not every command supports `--json` or `--db`.
Commands that read or write captured data accept `--db <path>` where supported,
including `run`, `serve`, reporting commands, `export`, and `inspect`.

The live tail is shown when `run` writes to a terminal on stderr. Use
`--no-live` for per-request log lines.

![Proxy with --no-live](demo/copilot-monitor-nolive.gif)

### Cost accuracy

Cost applies embedded per-token USD rates to captured usage. It does not know
your plan allocation, included credits, discounts, account billing rules, or
invoice total. Treat it as a directional engineering estimate, not billing
reconciliation. Unknown models use clearly flagged fallback pricing, and
requests without usage data cannot be priced accurately.

## Data and privacy

The default SQLite database is
`${XDG_DATA_HOME:-$HOME/.local/share}/copilot-monitor/store.db`. Override it
with `--db <path>`.

The normal database stores request metadata and token counts, not prompts,
completions, source code, or auth material. There is no monitoring cloud,
analytics, or phone-home; the proxy necessarily forwards the API request to the
upstream host you configured. `--raw-log <path>` is different: it writes
truncated request bodies and redacted response headers for debugging. Treat that
output as sensitive.

## Useful docs

[API and dashboard](docs/api.md) · [Architecture](docs/architecture.md) ·
[Homebrew](docs/homebrew.md) ·
[Project site](https://koopycat.github.io/copilot-monitor/)
