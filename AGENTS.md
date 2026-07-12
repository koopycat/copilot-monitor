# Repository Guidelines

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

Use the `justfile` as the main task runner:

- `just build` builds `./cmd/copilot-monitor` into `./copilot-monitor`.
- `just test` runs `go test ./...`.
- `just vet` runs `go vet`, `staticcheck`, and `govulncheck`.
- `just all` runs vet, tests, and build; use this before submitting changes.
- `just fmt` formats Go code with `go fmt ./...`.
- `just watch` starts hot reload with `air` for Go, HTML, and JavaScript
  changes.

Pre-commit hooks (configured in `.pre-commit-config.yaml`) run automatically on
`git commit` and enforce formatting (`gofmt`, `goimports`), `go mod tidy`,
`go vet`, and ESLint on the e2e tests. Tests, build, and the full e2e Playwright
suite run in CI instead. Install once with `pre-commit install`; bypass only
with a strong reason and `git commit --no-verify`.

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
- `openspec/specs/` contains the single source of truth for system behavior.
  Each capability has its own `spec.md` with requirements (SHALL/MUST) and
  WHEN/THEN scenarios. Do not include code paths, file paths, package names, or
  implementation plans there.
- `docs/` contains durable documentation for the current implementation, such as
  architecture, API behavior, operations, and troubleshooting. `docs/api.md` and
  `docs/architecture.md` are the technical reference pages served by GitHub
  Pages.
- `docs/index.html` is the GitHub Pages marketing landing page; it links to the
  technical docs and the GitHub repo.
- `openspec/changes/` is used for planning artifacts via the OpenSpec workflow.
  Use `/opsx:propose`, `/opsx:apply`, `/opsx:archive`. The legacy `plans/`
  directory may still exist but new planning goes through OpenSpec.
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
