## 1. Remove combined mode

- [x] 1.1 Remove `--dashboard` flag from `run.go`
- [x] 1.2 Remove `combinedDashProxy` function from `run.go`
- [x] 1.3 Remove `acceptsHTML` function from `run.go`
- [x] 1.4 Remove `dashboard` import and simplify handler setup in `run.go`
- [x] 1.5 Remove `--dashboard` completion entry
- [ ] 1.6 Remove `--routes-config` from `serve` completion (it was already
      there)

## 2. Specs and docs

- [x] 2.1 Remove "Browser-only routing in combined mode" from dashboard spec
- [x] 2.2 Update README to always show `serve` on separate port

## 3. Tests

- [x] 3.1 Verify existing tests pass
- [x] 3.2 Run full test suite with race detector
