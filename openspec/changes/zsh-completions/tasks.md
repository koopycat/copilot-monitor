## 1. CLI command

- [ ] 1.1 Create `internal/cli/completion.go` with a `runCompletion` function
      that dispatches on `zsh` subcommand
- [ ] 1.2 Add `"completion"` case to the root switch in `internal/cli/root.go`
- [ ] 1.3 Update `printUsage` in `root.go` to document the `completion` command

## 2. Zsh completion script

- [ ] 2.1 Write the zsh completion script as a Go string constant embedding all
      subcommands and their flags
- [ ] 2.2 Ensure `completion zsh` prints the script to stdout with exit code 0
- [ ] 2.3 Ensure `completion <unknown>` (e.g., `bash`) exits non-zero with an
      error message

## 3. Tests

- [ ] 3.1 Add a unit test for `completion zsh` verifying stdout contains
      expected subcommands and exit code 0
- [ ] 3.2 Add a unit test for `completion bash` verifying non-zero exit and
      error message

## 4. Documentation

- [ ] 4.1 Add a note in `AGENTS.md` that new subcommands or flags must also
      update the completion script
