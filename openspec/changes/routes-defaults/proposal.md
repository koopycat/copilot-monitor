## Why

Currently `copilot-monitor run` requires `--routes-config` and fails with an
error if omitted. This forces every user to create a routes JSON file before
their first run, even when the default GitHub Copilot monitoring setup covers
the most common use case. Moving to built-in defaults means
`copilot-monitor run` works out of the box with zero configuration.

## What Changes

- `--routes-config` becomes optional. When omitted, built-in default routes are
  used (GitHub Copilot API endpoints)
- When `--routes-config` is provided, user-defined routes **replace** the
  built-in defaults entirely (no merging)
- The startup banner shows whether built-in defaults or a config file is active
- The error message for missing `--routes-config` is removed
- A `copilot-monitor run --routes-config-defaults` flag prints the built-in
  default routes as JSON, so users can see what the defaults are and use them as
  a starting point for customization
- `examples/routes/github-copilot.json` remains as the canonical reference for
  the built-in defaults

## Capabilities

### New Capabilities

- `default-routes`: Built-in GitHub Copilot routes that activate when no
  `--routes-config` is provided. Covers `/chat/completions`, `/agents/*`,
  `/models`, `/models/session`, `/responses`, `/embeddings`, `/v1/engines/*`,
  `/v1/completions`, `/v1/messages/*`, and a `/_ping` local endpoint.

### Modified Capabilities

- `routing`: The "Missing routes config" requirement changes from "process
  fails" to "process loads built-in defaults". The `--routes-config-defaults`
  flag is added for inspection.

## Impact

- `internal/cli/run.go`: Remove required-check for `--routes-config`, use
  built-in defaults when absent, add `--routes-config-defaults` flag
- `internal/proxy/defaults.go` (new): Embedded route config with the Copilot
  defaults
- `internal/cli/completion.go`: Add `--routes-config-defaults` completion
- `openspec/specs/routing/spec.md`: Update "Missing routes config" scenario
