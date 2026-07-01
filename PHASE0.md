# Phase 0 Protocol Validation Spike

This spike is a small validation proxy for VSCode Copilot traffic.
It is not the final `copilot-monitor` CLI.
It does not write SQLite data, create sessions, calculate cost, or persist request bodies.

## Run

```sh
go run ./cmd/phase0
```

By default it listens on `127.0.0.1:7733` and prints a VSCode settings snippet.
Copy that snippet into your VSCode `settings.json`, then reload VSCode or restart the Copilot extension.

Useful flags:

```sh
go run ./cmd/phase0 --addr 127.0.0.1:7733 --log-bodies --max-body-log 2048
go run ./cmd/phase0 --proxy-unknown
```

## VSCode settings

The command prints this shape:

```json
{
  "github.copilot.advanced": {
    "debug.overrideProxyUrl": "http://127.0.0.1:7733",
    "debug.overrideCapiUrl": "http://127.0.0.1:7733",
    "authProvider": "github"
  }
}
```

## What to validate

Use Copilot Chat and inline completions while the command is running.
Watch stderr for request paths, redacted headers, whether an `Authorization` header arrived, parsed request model and stream flags, response status, streamed byte counts, response model events, and whether streamed SSE data contains `usage`.

Unknown paths return `502` by default.
Use `--proxy-unknown` to forward unknown paths to `https://api.githubcopilot.com` during exploration.
Phase 0 handles the observed `/_ping` health check locally and tunnels the observed `/responses` websocket path to `api.githubcopilot.com`.

If no traffic reaches the proxy after reloading VSCode, the first thing to test is the setting name casing.
The source-observed key is `debug.overrideCapiUrl`, but some community examples use `debug.overrideCAPIUrl`.
Try the alternate casing and record which one works with the installed extension version.

## Safety notes

Sensitive request header values are redacted in logs.
Request bodies are not logged unless `--log-bodies` is set.
If body logging is enabled, only the first `--max-body-log` bytes are printed.
