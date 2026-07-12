<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Automatic environment activation via direnv

The repo SHALL include a `.envrc` file that activates the devenv-managed
development environment when entering the repo directory. The activation SHALL
be transparent -- no manual command needed after initial direnv approval.

#### Scenario: Agent enters the repo directory

- **WHEN** an agent or developer runs `cd /path/to/copilot_monitoring`
- **THEN** the devenv environment is automatically activated
- **AND** Go 1.26, Node 24, pnpm, and other devenv-provided tools are available
  on PATH

#### Scenario: direnv is not installed

- **WHEN** a user without direnv enters the repo directory
- **THEN** a clear message is displayed explaining how to install direnv
- **AND** the user can still manually run `devenv shell`

### Requirement: Clean environment deactivation

When leaving the repo directory, the devenv environment SHALL be deactivated,
restoring the original PATH and environment variables.

#### Scenario: Agent leaves the repo directory

- **WHEN** an agent runs `cd ..` from the repo root
- **THEN** the devenv-provided tools are removed from PATH
- **AND** the original environment is restored
