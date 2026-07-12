<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Go-only fast build target

The repo SHALL provide a `just build-go` target that compiles all Go code with
`-tags nodashboard` and skips the dashboard build step. The target SHALL
complete in under 5 seconds on warmed caches.

#### Scenario: Agent builds after Go-only change

- **WHEN** an agent runs `just build-go` after modifying only Go source files
- **THEN** the Go binary is compiled successfully within 5 seconds
- **AND** no pnpm install or Svelte build is triggered

#### Scenario: Full build still available

- **WHEN** an agent or human runs `just build`
- **THEN** the dashboard is built and the complete binary is produced

### Requirement: Targeted test execution

The repo SHALL provide `just test-one <pattern>` that runs only Go tests
matching the given pattern across all packages. The repo SHALL provide
`just test-pkg <pkg>` that runs all tests in the specified package.

#### Scenario: Run single test by name

- **WHEN** an agent runs `just test-one TestStoreSessions`
- **THEN** only tests with names containing "TestStoreSessions" are executed

#### Scenario: Run tests in a specific package

- **WHEN** an agent runs `just test-pkg ./internal/store`
- **THEN** all tests in `./internal/store` are executed

### Requirement: Fast pre-commit hooks

Pre-commit hooks SHALL complete in under 3 seconds on average. Pre-commit SHALL
include only formatting (gofmt, goimports, prettier), secret scanning
(gitleaks), and whitespace checks. Slow checks (go vet, go mod tidy, dashboard
svelte-check) SHALL be enforced in CI instead.

#### Scenario: Agent commits a Go change

- **WHEN** an agent runs `git commit` after a Go-only change
- **THEN** the pre-commit hooks complete in under 3 seconds
- **AND** go vet, go mod tidy, and svelte-check are not executed

#### Scenario: CI catches what pre-commit no longer checks

- **WHEN** code is pushed that would fail `go vet` or `go mod tidy`
- **THEN** the CI workflow fails and reports the violation
