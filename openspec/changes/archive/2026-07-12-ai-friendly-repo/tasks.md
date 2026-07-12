## 1. Fast build and test targets

- [x] 1.1 Add `build-go` target to justfile:
      `go build -tags nodashboard -o ./bin/copilot-monitor ./cmd/copilot-monitor`
- [x] 1.2 Add `test-one <pattern>` target:
      `go test -tags nodashboard -run <pattern> ./...`
- [x] 1.3 Add `test-pkg <pkg>` target: `go test -tags nodashboard <pkg>`
- [x] 1.4 Add `test-all` target (no filtering):
      `go test -tags nodashboard ./...`
- [x] 1.5 Reorganize justfile with section headers: Build, Test, Check, Format,
      Dev

## 2. Speed up pre-commit

- [x] 2.1 Remove `go-vet` hook from pre-commit config
- [x] 2.2 Remove `go-mod-tidy` hook from pre-commit config
- [x] 2.3 Remove `dashboard-check` hook from pre-commit config
- [x] 2.4 Verify remaining hooks (formatting, secrets, whitespace) complete in
      <3s

## 3. Absorb checks into CI

- [x] 3.1 Add `go vet` step to CI lint job (if not present; it already is)
- [x] 3.2 Add `go mod tidy -diff` step to CI lint job (if not present; it
      already is)
- [x] 3.3 Add `dashboard svelte-check` step to CI lint job
- [x] 3.4 Verify CI catches violations that pre-commit no longer checks

## 4. Environment automation

- [x] 4.1 Create `.envrc` that activates devenv environment
- [x] 4.2 Add friendly error message when direnv is not installed
- [x] 4.3 Add `.envrc` to git (it contains no secrets, only devenv activation)
- [x] 4.4 Verify `cd` into repo activates the environment correctly

## 5. Agent documentation

- [x] 5.1 Add "Quick Reference" section to top of AGENTS.md with common
      workflows
- [x] 5.2 Document prerequisites (direnv, devenv, pre-commit) and `just setup`
- [x] 5.3 Document the fast build workflow: `just build-go` and `just test-one`
- [x] 5.4 Document that pre-commit is formatting-only; vet/tidy are CI-enforced

## 6. Validation

- [x] 6.1 Run `just build-go` and confirm <5s on warm cache
- [x] 6.2 Run `just test-one TestStoreSessions` and confirm targeted execution
- [x] 6.3 Run `just all` and confirm no regressions
- [x] 6.4 Run `git commit --dry-run` and confirm pre-commit runs in <3s
