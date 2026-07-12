## Why

The CLI currently has no shell completion support. Users must memorize or look
up every subcommand, flag, and argument. Zsh completions would let users
tab-complete commands, flags, and values, reducing friction and typos. Zsh is
the primary shell for the maintainer and a common default on macOS.

## What Changes

- New `copilot-monitor completion zsh` subcommand that outputs a zsh completion
  script to stdout
- The completion script covers all top-level subcommands, their flags, and flag
  values where appropriate (e.g., `--log-format` accepts `human` or `json`)
- Users source the output in their `.zshrc` to enable completions

## Capabilities

### New Capabilities

- `shell-completions`: Generate zsh shell completion scripts for the
  copilot-monitor CLI

### Modified Capabilities

<!-- None -->

## Impact

- Affected code: `internal/cli/` (new `completion.go`),
  `cmd/copilot-monitor/main.go` (no change needed -- dispatched via root switch)
- No dependency changes (hand-rolled completion script, no library needed)
- No API or config changes
- No breaking changes
