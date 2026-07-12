## 1. Config and routing

- [ ] 1.1 Add `RouteCompression` struct to `internal/proxy/config.go`
- [ ] 1.2 Add `Compression` field to `RouteConfig` and `Route`
- [ ] 1.3 Wire `Compression` through `NewRouter` for both exact and
      provider-default routes

## 2. Proxy handler changes

- [ ] 2.1 Replace `compressor` and `compressionRequired` fields with lazy client
      cache in Handler
- [ ] 2.2 Remove `ConfigureCompression` method
- [ ] 2.3 Add `getCompressor` method with lazy construction and caching per
      endpoint
- [ ] 2.4 Update `maybeCompress` to use route-level compression config
- [ ] 2.5 Update `compressionEligible` to check route config instead of handler
      field

## 3. CLI cleanup

- [ ] 3.1 Remove all 5 `--headroom-*` flag definitions from `run.go`
- [ ] 3.2 Remove headroom client initialization block from `run.go`
- [ ] 3.3 Remove `ConfigureCompression` call and startup message
- [ ] 3.4 Remove `compressionMode` helper function
- [ ] 3.5 Remove headroom completion entries from `completion.go`

## 4. Tests

- [ ] 4.1 Add `SetCompressor` test helper to `testhelpers.go`
- [ ] 4.2 Update `compression_test.go` to use route-level compression config
- [ ] 4.3 Update `integration/compression_test.go` harness and call sites
- [ ] 4.4 Remove `TestCompressionMode` from `cli_test.go`
- [ ] 4.5 Run full test suite (`go test ./...`, integration tests, go vet)

## 5. Documentation

- [ ] 5.1 Add commented compression example to
      `examples/routes/github-copilot.json`
- [ ] 5.2 Update `openspec/specs/compression/spec.md` via archive step
