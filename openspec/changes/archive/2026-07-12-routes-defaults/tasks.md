## 1. Built-in defaults

- [x] 1.1 Add `internal/proxy/defaults.go` with `DefaultRoutes()` returning a
      `*ProxyConfig` matching `examples/routes/github-copilot.json`
- [x] 1.2 Add `--routes-config-defaults` flag to `run` that prints default
      routes as JSON and exits
- [x] 1.3 Update `run.go` to use `DefaultRoutes()` when `--routes-config` is
      empty (remove the required-check and error)
- [x] 1.4 Update startup banner to show "using built-in default routes" vs
      "using routes from config file"
- [x] 1.5 Add `--routes-config-defaults` completion to `completion.go`

## 2. Tests

- [x] 2.1 Test that `copilot-monitor run` without `--routes-config` starts
      successfully with default routes
- [x] 2.2 Test that `--routes-config` overrides defaults (default routes are not
      loaded)
- [x] 2.3 Test that `--routes-config-defaults` prints valid JSON matching the
      example file
- [x] 2.4 Test that the startup banner reflects the active route source
- [x] 2.5 Run full test suite (`go test ./...`, integration tests, go vet)

## 3. Spec update

- [x] 3.1 Update `openspec/specs/routing/spec.md` with the modified "Missing
      routes config" scenario (archive step)
