## Context

AI coding agents are primary contributors to this repo. The current tooling was
designed for human developers who can context-switch cheaply and tolerate
multi-second feedback loops. Agents have different cost characteristics: every
second of build time, every unnecessary file read, and every unclear convention
consumes tokens and degrades output quality.

Current friction points:

- `just build` always rebuilds the dashboard (pnpm install + pnpm build), adding
  10-20s to Go-only changes.
- Pre-commit runs `go vet ./...`, `go mod tidy -diff`, and
  `dashboard svelte-check` -- all slow operations that block `git commit` for
  5-15s.
- No direnv integration; agents (and humans) must remember to run `devenv shell`
  before any command.
- The justfile is a flat list of targets with no logical grouping, making it
  hard for agents to discover the right command.
- AGENTS.md lacks a quick-reference section with common agent workflows.

## Goals / Non-Goals

**Goals:**

- Give agents a <3s build-and-test feedback loop for Go-only changes.
- Keep pre-commit under 3s wall-clock time.
- Automate environment activation so agents never hit "command not found" on
  first run.
- Structure the justfile so agents can quickly find the right target without
  reading the entire file.
- Give AGENTS.md a quick-reference section agents can read in a single context
  window.

**Non-Goals:**

- Changing how the dashboard is built or served.
- Modifying Go project layout or package structure.
- Adding new dependencies.
- Changing CI beyond absorbing checks moved from pre-commit.
- Automating anything beyond environment setup (no AI-specific CI, no automated
  PR descriptions).

## Decisions

### Decision 1: Use `just build-go` as the fast build target, not `go build` directly

`just build-go` keeps all build commands discoverable under one tool. It uses
`-tags nodashboard` to skip Svelte compilation while still producing a working
binary (tests embed a stub). Agents learn one interface (`just`) instead of
mixing `just` and raw `go` invocations.

Alternatives considered:

- Teach agents to run `go build -tags nodashboard ./...` -- fragile, agents may
  forget the tag.
- Make `just build` skip dashboard by default and add `just build-all` -- breaks
  muscle memory for humans who expect `just build` to produce a complete binary.

### Decision 2: Move slow checks to CI, keep pre-commit to formatting + secrets only

Pre-commit speed is critical for agent iteration. Formatting (gofmt, goimports,
prettier) and gitleaks are fast (<2s total). `go vet`, `go mod tidy`, and
`svelte-check` are slow (5-15s) and already run in CI. They add no value when
run locally on every commit if CI blocks the merge anyway.

Keeping `go vet` in CI is sufficient because:

1. CI runs on every push to a PR branch.
2. The agent can run `just vet` explicitly before pushing if desired.
3. The cost of a CI failure (re-push) is lower than the cumulative cost of 5-15s
   per commit across dozens of commits per session.

Alternatives considered:

- Keep vet in pre-commit but run `go vet ./pkg/...` only on changed packages --
  complex, fragile.
- Run vet in a non-blocking pre-push hook -- not universally configured, adds
  inconsistency.

### Decision 3: Use direnv with `.envrc` that sources devenv

direnv is the standard tool for per-directory environment activation. The
`.envrc` calls `devenv shell` (or `use flake` with the devenv flake output) to
activate the nix environment. direnv auto-loads on `cd` and unloads on `cd ..`,
so agents always have Go 1.26, Node 24, and pnpm available.

The `.envrc` includes a guard that checks for direnv and prints a friendly
message if missing.

Alternatives considered:

- `nix-direnv` with `use flake` directly -- requires nix-direnv to be installed,
  adds a dependency.
- `devenv shell` in `.envrc` -- simple, works with standard direnv.

### Decision 4: Group justfile targets with comment headers

Use `# ---` as section separators with descriptive headers like `# Build`,
`# Test`, `# Check`, `# Format`, `# Dev`. This lets agents scan the file
structure by reading only the header lines.

Alternatives considered:

- Separate justfiles (`justfile.build`, `justfile.test`) -- `just` doesn't
  natively support includes without `import`.
- Separate commands (`make`, `task`) -- fractures the tooling surface.

### Decision 5: `just test-one` uses Go test patterns, not package paths

`just test-one TestStoreSessions` runs `go test ./... -run TestStoreSessions`.
This is more ergonomic for agents who know the test name but not the exact
package path.

Also provide `just test-pkg <pkg>` for package-level targeting:
`just test-pkg ./internal/store`.

## Risks / Trade-offs

- **Risk**: Moving `go vet` out of pre-commit lets vet failures reach CI. →
  **Mitigation**: CI runs vet on every push. The agent can run `just vet` before
  pushing. Cost of a CI re-push is lower than cumulative pre-commit latency.

- **Risk**: `just build-go` uses `-tags nodashboard` which stubs the dashboard
  embed. An agent might forget to do a full build before testing dashboard
  changes. → **Mitigation**: Document this clearly in AGENTS.md quick-reference.
  The `just build` (full) target remains available.

- **Risk**: direnv may not be installed on all machines. Agents or users new to
  the repo may hit "command not found". → **Mitigation**: `.envrc` prints a
  clear message pointing to direnv installation if the tool is missing.
  AGENTS.md quick-reference lists it as a prerequisite.

- **Risk**: Grouped justfile sections add visual noise for humans who already
  know the targets. → **Mitigation**: The section headers are 2-line comments,
  minimal visual overhead. They also help humans discover targets.

## Open Questions

- None. All decisions above are settled and ready for implementation.
