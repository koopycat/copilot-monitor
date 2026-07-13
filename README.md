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

This path requires a separately installed and authenticated `pi` client. Set up
the Kilo routes once. The default configuration path is
`${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor/routes.json`.

```sh
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor"
cp examples/routes-kilo.json "${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor/routes.json"
copilot-monitor validate --routes-config "${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor/routes.json"
```

In terminal 1, start the proxy and dashboard:

```sh
copilot-monitor run --dashboard
```

In terminal 2, verify both services, make one request through the proxy, then
inspect the capture:

```sh
curl http://127.0.0.1:7733/_health
curl http://127.0.0.1:7734/api/health
KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi -p 'Reply OK'
copilot-monitor stats --since 1h
```

## Client setup and routes

Built-in routes cover GitHub Copilot. A valid default routes file auto-loads
from `${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor/routes.json`; an
explicit `--routes-config <file>` replaces that default configuration.

VS Code is not auto-detected. To route GitHub Copilot through the proxy, set
`github.copilot.advanced.debug.overrideCapiUrl` to
`http://127.0.0.1:7733/copilot` in VS Code settings, then run
`copilot-monitor run`.

A client base URL must match the routes it will request after its provider
prefix is removed. For example, the Kilo base URL `http://127.0.0.1:7733/kilo`
matches the `/chat/completions` route in
[`examples/routes-kilo.json`](examples/routes-kilo.json).

Choose a route file that matches the client's API paths, validate it, then pass
it explicitly to `run`:

| Route example                                                      | Route path after prefix stripping | Matching proxy base URL           |
| ------------------------------------------------------------------ | --------------------------------- | --------------------------------- |
| [`examples/routes/openai.json`](examples/routes/openai.json)       | `/v1/chat/completions`            | `http://127.0.0.1:7733/openai/v1` |
| [`examples/routes/anthropic.json`](examples/routes/anthropic.json) | `/v1/messages`                    | `http://127.0.0.1:7733/anthropic` |

```sh
copilot-monitor validate --routes-config examples/routes/openai.json
copilot-monitor run --routes-config examples/routes/openai.json
```

`init` is optional. It only creates a starter OpenAI/Anthropic routes file,
using the `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` environment keys when
present; it is not a full client configuration. It writes to the default route
path and refuses to overwrite it without `--force`. Built-in routes remain
available for Copilot when no valid default config or explicit route file is
used.

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
| `init`       | Create a starter routes file from environment keys.                          |
| `validate`   | Validate a routes configuration file.                                        |
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
reconciliation. Fallback pricing, routes marked not billed, and requests with
missing usage data reduce its accuracy.

## Data and privacy

The default SQLite database is
`${XDG_DATA_HOME:-$HOME/.local/share}/copilot-monitor/store.db`. Override it
with `--db <path>`. The default routes file is
`${XDG_CONFIG_HOME:-$HOME/.config}/copilot-monitor/routes.json`.

The normal database stores request metadata and token counts, not prompts,
completions, source code, or auth material. `--raw-log <path>` is different: it
writes truncated request bodies and redacted response headers for debugging.
Treat that output as sensitive.

## Useful docs

[API and dashboard](docs/api.md) · [Architecture](docs/architecture.md) ·
[Homebrew](docs/homebrew.md) · [Kilo routes](examples/routes-kilo.json) ·
[OpenAI routes](examples/routes/openai.json) ·
[Anthropic routes](examples/routes/anthropic.json) ·
[Project site](https://koopycat.github.io/copilot-monitor/)
