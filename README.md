# Copilot Monitor

[![CI](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/koopycat/copilot-monitor)](https://github.com/koopycat/copilot-monitor/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**See the tokens, latency, and estimated cost behind your AI-tool traffic.**

Copilot Monitor is a local HTTP proxy and SQLite-backed dashboard for LLM API
traffic. It records request metadata, token usage, latency, and estimated cost
on your machine. No cloud or telemetry. By default, the proxy listens only on
`127.0.0.1`; the database does not store prompts, completions, source code, or
auth headers.

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
brew trust koopycat/copilot-monitor
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
```

## Client setup

Point your client's base URL to the proxy address (`http://127.0.0.1:7733`). All
traffic is forwarded to the configured `--upstream` host.

VS Code is not auto-detected. To route GitHub Copilot through the proxy, set
`github.copilot.advanced.debug.overrideCapiUrl` to `http://127.0.0.1:7733` in VS
Code settings, then run:

```sh
copilot-monitor run --upstream api.githubcopilot.com
```

## Commands

| Command      | Purpose                                                                      |
| ------------ | ---------------------------------------------------------------------------- |
| `run`        | Start the local proxy; `--dashboard` also starts the dashboard on port 7734. |
| `serve`      | Start the dashboard and read-only API from an existing database.             |
| `stats`      | Show captured usage by model and endpoint.                                   |
| `cost`       | Show estimated equivalent provider list-price cost.                          |
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
Not every command supports `--json` or `--db`. Commands that read or write
captured data accept `--db <path>` where supported, including `run`, `serve`,
reporting commands, `export`, and `inspect`.

The live tail is shown when `run` writes to a terminal on stderr. Use
`--no-live` for per-request log lines.

![Proxy with --no-live](demo/copilot-monitor-nolive.gif)

### Cost accuracy

Cost is an embedded equivalent provider list-price estimate, not invoice
reconciliation. Fallback pricing and requests with missing usage data reduce its
accuracy.

## Data and privacy

The default SQLite database is
`${XDG_DATA_HOME:-$HOME/.local/share}/copilot-monitor/store.db`. Override it
with `--db <path>`.

The normal database stores request metadata and token counts, not prompts,
completions, source code, or auth material. `--raw-log <path>` is different: it
writes truncated request bodies and redacted response headers for debugging.
Treat that output as sensitive.

## Useful docs

[API and dashboard](docs/api.md) · [Architecture](docs/architecture.md) ·
[Homebrew](docs/homebrew.md) ·
[Project site](https://koopykat.github.io/copilot-monitor/)
