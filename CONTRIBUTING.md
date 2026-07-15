# Contributing to Copilot Monitor

## Prerequisites

Install [direnv](https://direnv.net/), [devenv](https://devenv.sh/), and
[pre-commit](https://pre-commit.com/). The environment auto-activates via
direnv. Run `pre-commit install` once per clone.

## Getting started

```sh
git clone https://github.com/koopycat/copilot-monitor.git
cd copilot-monitor
just setup
direnv allow
```

`just setup` installs Go tools, Node packages and the pre-commit hooks. The
devenv shell provides Go 1.26, Node 24, and pnpm.

## Making changes

**Build:** `just build-go` (fast, skips dashboard) or `just build` (full).

**Test:** `just test` (fast unit), `just test-all` (every Go package). Target
specific tests with `just test-one TestName` or `just test-pkg ./pkg`. Use
`just integration` for HTTP-level tests and `just e2e` for browser tests.

**Check:** `just vet` runs go vet, staticcheck, and govulncheck.

**Format:** `just format` (Go, JS, Svelte, Markdown, JSON, YAML) or
`just fmt-go` for Go only.

## Before submitting

Run `just all`. It runs vet, secret scanning, fast unit tests, and a full build
— the same checks that gate CI. Also run `just format` to keep diffs clean.

## Commit style

Single-line, imperative summary. Examples:

- `Add request-id header to proxy`
- `Fix export help and add export tests`

Keep commits scoped and describe the behavior changed.

## Pull request process

CI checks include lint and test, plus trivy, osv-scanner, and gitleaks security
scanning. All status checks must pass before merge. `main` has branch
protection.

For dashboard changes, include a screenshot or a short description of the visual
change so reviewers can assess it without building locally.

When behavior changes, update the relevant requirements in `openspec/specs/` and
the durable docs in `docs/`.
