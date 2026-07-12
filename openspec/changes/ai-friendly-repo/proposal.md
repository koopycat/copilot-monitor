## Why

AI coding agents (pi, Claude Code, GitHub Copilot, etc.) are primary
contributors to this repo, but the current tooling and conventions create
friction that wastes agent context windows and compute tokens. Every unnecessary
dashboard build, every slow pre-commit hook, and every unclear convention costs
real money and degrades output quality. Treating AI agents as first-class users
of the development tooling unlocks faster, cheaper, and higher-quality
contributions.

## What Changes

- Add a Go-only fast build target (`just build-go`) that skips the dashboard
  build, cutting the common agent feedback loop from ~20s to ~2s.
- Speed up pre-commit by moving slow checks (go vet, go mod tidy, dashboard
  svelte-check) to CI-only and keeping pre-commit to instant operations
  (formatting, secret scan).
- Add `.envrc` for direnv to auto-activate the devenv environment on `cd`,
  eliminating the manual `devenv shell` step.
- Restructure `justfile` with grouped targets so agents can quickly discover the
  right command (build, test, check, format).
- Expand AGENTS.md with a quick-reference section covering common agent
  workflows: fast build, targeted tests, formatting before commit.
- Add `just test-one <pkg>` and `just test-one name` pattern matching for
  targeted test runs.
- Ensure Go build cache is properly shared and persisted across invocations so
  repeated agent builds are instant.

## Capabilities

### New Capabilities

- `fast-feedback`: Build and test targets optimized for rapid AI-driven
  development loops, including Go-only builds and targeted test execution.
- `agent-documentation`: Repo conventions and instructions structured for AI
  agent consumption, with quick-reference workflows and predictable patterns.
- `environment-automation`: Automatic development environment activation via
  direnv, eliminating manual setup steps on repo entry.

### Modified Capabilities

<!-- No existing spec requirements change. These are new DX capabilities. -->

## Impact

- `justfile`: Restructured with new targets and logical grouping.
- `.pre-commit-config.yaml`: Slow hooks moved to CI; pre-commit latency reduced
  to <3s.
- `.github/workflows/ci.yml`: Absorbs vet, go mod tidy, and dashboard-check from
  pre-commit.
- `.envrc`: New file for direnv auto-activation.
- `AGENTS.md`: Expanded with quick-reference section.
- No API, database schema, or user-facing product changes.
