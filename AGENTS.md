# Repository Guidelines

## Quick Reference

Use `just` for all development commands and run it from the repository root. Run
`just --list` to discover available recipes; the `justfile` is the authoritative
list of supported commands. The environment auto-activates via direnv.

**Prerequisites:** direnv, devenv, pre-commit. Run `just setup` on the first
clone.

**Build:**

- `just build-go` -- fast Go-only build with the dashboard skipped
- `just build` -- full build including dashboard assets

**Test:**

- `just test` -- fast unit tests, excluding integration tests
- `just test-one TestName` -- run tests matching a name pattern
- `just test-pkg ./internal/store` -- run all tests in a package
- `just test-all` -- run all Go tests across every package
- `just integration` -- run HTTP-level integration tests
- `just e2e` -- run browser end-to-end tests
- `just all` -- run vet, secret scanning, fast unit tests, and the full build;
  excludes integration and end-to-end tests

**Check:**

- `just vet` -- run go vet, staticcheck, and govulncheck
- `just secrets` -- run the fast secret scan

**Format:**

- `just format` -- format Go, JavaScript, Svelte, Markdown, JSON, and YAML
- `just fmt-go` -- format Go code only

Pre-commit runs formatting, secrets, and dashboard svelte-check. Slow Go checks
(`go vet`, `go mod tidy`) and svelte-check are enforced in CI as the
comprehensive safety net.

**Notes:**

- TypeScript is pinned to ~6.0 in dashboard because svelte-check 4.x crashes on
  TS 7 (typescript.sys undefined). Revisit when svelte-check 5 is available.

## Project Structure & Module Organization

This is a Go CLI and local proxy for monitoring GitHub Copilot API usage. The
executable entry point is `cmd/copilot-monitor/main.go`; application code lives
under `internal/`. Key packages are `internal/cli` for commands,
`internal/proxy` for forwarding and capture, `internal/api` for the dashboard
API, `internal/store` for SQLite persistence, `internal/cost` and
`internal/catalog` for pricing/model data, and `internal/log` for terminal
output. Dashboard assets live in `dashboard/`; the schema is
`internal/store/schema.sql`.

## Build, Test, and Development Commands

The development environment is managed by **devenv** (`devenv.nix`). It provides
Go 1.26, Node.js 24, and pnpm 11.

Use the `justfile` as the canonical task runner rather than invoking the
underlying Go, Node.js, or pnpm commands directly. Run `just --list` when a
recipe is not listed in this document.

For local use, run `./copilot-monitor serve` for the dashboard API or
`./copilot-monitor run` for the proxy.

## Coding Style & Naming Conventions

Follow standard Go style: tabs via `gofmt`, small packages, explicit error
handling, and table-driven tests where useful. Keep command handlers in
`internal/cli` named by command, for example `stats.go` and `runStats`. When
adding a new subcommand or changing a command's flags, update the zsh completion
script in `internal/cli/completion.go`. Keep HTTP/API handlers grouped by
feature in `internal/api` and proxy behavior in `internal/proxy`. JavaScript and
CSS in `internal/dashboard` should remain dependency-light and colocated with
`index.html`.

## Agentic Workflows

- `README.md` is for user-facing setup, smoke tests, and common commands.
- `SPEC.md` is an index. Requirements live in
  `openspec/specs/<capability>/spec.md`.
- `openspec/specs/` is the authoritative single source of truth for system
  behavior. Each capability has its own `spec.md` with requirements (SHALL/MUST)
  and WHEN/THEN scenarios. Do not include code paths, file paths, package names,
  or implementation plans there.
- `docs/` contains durable documentation for the current implementation, such as
  architecture, API behavior, operations, and troubleshooting. `docs/api.md` and
  `docs/architecture.md` are the technical reference pages served by GitHub
  Pages.
- `docs/index.html` is the GitHub Pages marketing landing page; it links to the
  technical docs and the GitHub repo.
- `openspec/changes/` is used for planning artifacts via the OpenSpec workflow.
  Its lifecycle is explore, propose, update, apply, sync, and archive; use the
  corresponding OpenSpec skill for the detailed instructions. Sync affected
  delta specs into `openspec/specs/` before archiving, and archive selected
  changes under `openspec/changes/archive/YYYY-MM-DD-<change-name>/`. Never
  auto-select a change for archiving, and do not archive incomplete work without
  explicit user authorization. The legacy `plans/` directory may still exist,
  but new planning goes through OpenSpec.
- `openspec/config.yaml` contains project context and non-behavioral design
  constraints (quality attributes, naming conventions, DX principles).

GitHub Pages is served from `/docs` on the default branch. The landing page is
at `docs/index.html` (URL: `/`). Technical docs are at `docs/api.md` and
`docs/architecture.md` (URLs: `/api` and `/architecture` via Jekyll's `.md`
extension stripping).

- `PRODUCT.md` contains product intent, audience, and design principles.
- Do not add temporary output, such as ad hoc implementation notes or scratch
  plans, to `docs/` or `openspec/specs/`.
- Clean up temporary outputs after finishing a task.
- When behavior changes, update requirements, durable docs, and active plans
  only when each is actually affected.

## Testing Guidelines

Tests use the standard Go testing package. Place tests beside implementation
files as `*_test.go`, with names like `TestStoreSessions` or
`TestRouterCapturesUsage`. Prefer unit tests for parsers, cost calculations,
storage, and CLI output, and integration-style tests for proxy/API behavior when
HTTP semantics matter. Run `just test` during development and `just all` before
opening a PR.

## Commit & Pull Request Guidelines

Recent commits use short, imperative summaries such as
`Split api.go by handler group` and `Fix export help and add export tests`. Keep
commits scoped and describe the behavior changed. Pull requests should include a
concise summary, test results (`just all` output or noted exceptions), linked
issues when applicable, and screenshots or notes for dashboard/UI changes.

## Security & Configuration Tips

Do not store prompts, completions, source code, auth headers, cookies, or API
keys. Preserve the loopback-only default addresses and avoid expanding proxy
exposure without an explicit security rationale. Keep local data paths
configurable through existing flags such as `--db`, `--addr`, and
`--usage-debug-log`.
