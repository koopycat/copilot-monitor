# Copilot Monitor

[![CI](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/koopycat/copilot-monitor/actions/workflows/ci.yml)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/koopycat/copilot-monitor)](https://github.com/koopycat/copilot-monitor/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Know exactly what your AI tools are costing you.**

A local proxy that captures per-request metadata, token counts, latency, and
estimated cost from LLM API traffic. Everything stored in SQLite on your
machine. No cloud. No telemetry. No prompts, completions, or auth headers are
ever written to disk.

Works with GitHub Copilot out of the box, plus pi-agent, Claude Code, aider, and
any OpenAI- or Anthropic-compatible API.

```sh
./copilot-monitor run --dashboard &
# → proxy on http://127.0.0.1:7733, dashboard on http://127.0.0.1:7734
```

[Project site](https://koopycat.github.io/copilot-monitor/)

## What it looks like

![CLI overview](demo/copilot-monitor.gif)

_Reporting commands against captured data (synthetic). Open
`http://127.0.0.1:7734` for the full dashboard._

## Download

| Platform       | Link                                                                                                                                            |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| macOS ARM64    | [copilot-monitor-darwin-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-arm64.tar.gz) |
| macOS Intel    | [copilot-monitor-darwin-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-darwin-amd64.tar.gz) |
| Linux x86_64   | [copilot-monitor-linux-amd64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-amd64.tar.gz)   |
| Linux ARM64    | [copilot-monitor-linux-arm64.tar.gz](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-linux-arm64.tar.gz)   |
| Windows x86_64 | [copilot-monitor-windows-amd64.zip](https://github.com/koopycat/copilot-monitor/releases/latest/download/copilot-monitor-windows-amd64.zip)     |

Or build from source:

```sh
devenv shell
just build
```

## Commands

```sh
copilot-monitor stats --since 7d
copilot-monitor cost --since 7d
copilot-monitor today
copilot-monitor live
copilot-monitor live --watch
copilot-monitor sessions --since 7d
```

All commands support `--json` and `--db <path>`.

## Using with other tools

Start the proxy, then point your tools at it:

```sh
./copilot-monitor run

# pi-agent
KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi

# OpenAI-compatible tools
OPENAI_BASE_URL=http://127.0.0.1:7733/openai/v1

# Copilot in VS Code — auto-detected, no config needed
```

A live session tail refreshes every 2 seconds. Pass `--no-live` for per-request
log lines:

![Proxy with --no-live](demo/copilot-monitor-nolive.gif)

Configurable routes, compression, and model policy are documented on the
[project site](https://koopycat.github.io/copilot-monitor/).

## Privacy

All data stays on your machine. The proxy binds to `127.0.0.1` only. Nothing
uploaded. The database stores metadata and token counts — never prompts,
completions, source code, or auth material.
