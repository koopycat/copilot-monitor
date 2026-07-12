## Why

After the `routes-defaults` change, `copilot-monitor run` works with built-in
Copilot defaults. But users who customized their routes (e.g., added Kilo
routes) must remember to pass
`--routes-config ~/.config/copilot-monitor/routes.json` every time. The `init`
command already creates a routes file at that exact path. The proxy should look
there automatically.

## What Changes

- When `--routes-config` is not provided, try loading
  `$XDG_CONFIG_HOME/copilot-monitor/routes.json` (falling back to
  `~/.config/copilot-monitor/routes.json`) before falling back to built-in
  defaults
- If the config file exists and is valid, use it. If it doesn't exist or fails
  to parse, fall back to built-in defaults (never fail on a missing default
  file)
- Startup banner reflects which source is active: built-in defaults, config file
  at default path, or explicit `--routes-config`
- The `init` command's output message remains: it already advises
  `copilot-monitor run --routes-config <path>`, which still works; users can
  also simply run `copilot-monitor run` after `init`

## Capabilities

### Modified Capabilities

- `default-routes`: The "Default routes activate when no config provided"
  scenario changes from "uses built-in defaults" to "tries XDG config file
  first, falls back to built-in defaults"

## Impact

- `internal/cli/run.go`: Add default config path resolution between "no flag"
  and built-in defaults
- `internal/store/store.go`: May export `XDGConfigDir()` or similar
- `openspec/specs/default-routes/spec.md`: Update scenario
