## 1. Sidecar implementation

- [x] 1.1 Add `--dashboard` flag back to `run` (description: "start dashboard on
      separate port 7734")
- [x] 1.2 Start dashboard server in goroutine when flag is set
- [x] 1.3 Wire shutdown so both listeners stop on SIGINT/SIGTERM
- [x] 1.4 Add `--dashboard` completion back

## 2. Docs and specs

- [x] 2.1 Add `--dashboard` back to README flags table
- [x] 2.2 Add "Dashboard sidecar" requirement to
      openspec/specs/dashboard/spec.md (archive step)

## 3. Tests

- [x] 3.1 Run full test suite with race detector
