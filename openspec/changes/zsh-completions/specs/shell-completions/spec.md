## ADDED Requirements

### Requirement: Zsh completion subcommand

The CLI SHALL provide a `completion zsh` subcommand that outputs a zsh
completion script to stdout. The script SHALL enable tab-completion for all
top-level subcommands and, where applicable, their flags and flag values.

#### Scenario: Generate zsh completions

- **WHEN** the user runs `copilot-monitor completion zsh`
- **THEN** a valid zsh completion script is printed to stdout
- **AND** the exit code is 0

#### Scenario: Tab-complete top-level commands

- **WHEN** the generated script is sourced and the user types `copilot-monitor `
  followed by TAB
- **THEN** zsh offers completions for all top-level subcommands: `run`, `serve`,
  `stats`, `cost`, `today`, `sessions`, `live`, `export`, `init`, `validate`,
  `inspect`, `version`, `help`, `completion`

#### Scenario: Tab-complete flag for command with flags

- **WHEN** the generated script is sourced and the user types
  `copilot-monitor run --` followed by TAB
- **THEN** zsh offers the flags accepted by `run`: `--routes-config`, `--addr`,
  `--db`, `--project`, `--usage-debug-log`, `--no-live`, `--dashboard`,
  `--log-format`

#### Scenario: Completion does not require database or config files

- **WHEN** `copilot-monitor completion zsh` is run
- **THEN** no database connection, config file, or network access is required
- **AND** the command succeeds regardless of the current working directory

#### Scenario: Unknown subcommand to completion

- **WHEN** the user runs `copilot-monitor completion bash`
- **THEN** the CLI exits with a non-zero code and prints an error indicating
  that only `zsh` is supported

### Requirement: Completion script is self-contained

The generated zsh completion script SHALL be a single, self-contained file with
no external dependencies. It SHALL follow the standard zsh completion convention
of defining a `_copilot-monitor` function and registering it with `compdef`.

#### Scenario: Sourced in .zshrc

- **WHEN** the user adds `source <(copilot-monitor completion zsh)` to their
  `.zshrc`
- **THEN** completions are active in all new shell sessions

### Requirement: Composable output

The `completion zsh` subcommand SHALL write the script to stdout without any
banners, warnings, or help text, so the output can be redirected to a file or
piped.

#### Scenario: Output redirected to file

- **WHEN** the user runs `copilot-monitor completion zsh > _copilot-monitor`
- **THEN** the file `_copilot-monitor` contains only the valid completion script
  with no extra output
