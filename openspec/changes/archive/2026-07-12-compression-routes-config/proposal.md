## Why

Headroom compression currently requires 5 separate CLI flags (`--headroom-url`,
`--headroom-timeout`, `--headroom-required`,
`--headroom-compress-user-messages`, `--headroom-target-ratio`). This scatters
compression configuration across the command line when it logically belongs with
route definitions — compression is route-contextual (only applies to
`/chat/completions`), and the routes JSON is already the single source of truth
for upstream behavior.

## What Changes

- **BREAKING**: Remove all 5 `--headroom-*` CLI flags from `copilot-monitor run`
- Add `compression` config block to route definitions in the routes JSON file
- Compression is configured per-route, enabling different compression settings
  for different providers or paths
- Timeout is hardcoded to 30 seconds (loopback call, never needs tuning)
- Headroom clients are lazily constructed and cached per endpoint
- Remove headroom-related tab completion entries

## Capabilities

### New Capabilities

<!-- No new capabilities — same compression feature, different configuration method -->

### Modified Capabilities

- `compression`: Configuration moves from CLI flags to a `compression` block in
  the routes JSON. Compression is now per-route. Timeout is no longer
  configurable (hardcoded to 30s). Required mode is now `compression.required`
  instead of `--headroom-required`. Compress-user-messages and target-ratio move
  to the config block.

## Impact

- `internal/cli/run.go`: Remove headroom flag definitions and initialization
- `internal/cli/completion.go`: Remove headroom completion entries
- `internal/proxy/config.go`: Add `RouteCompression` struct and field
- `internal/proxy/router.go`: Add `Compression` to `Route`, wire in `NewRouter`
- `internal/proxy/server.go`: Replace single compressor with lazy client cache
- `internal/proxy/compression.go`: Remove `ConfigureCompression`, add
  `getCompressor`, update `maybeCompress`/`compressionEligible`
- `internal/proxy/testhelpers.go`: Add `SetCompressor` test helper
- `internal/proxy/compression_test.go`: Update to route-level config
- `internal/integration/compression_test.go`: Update harness and call sites
- `internal/cli/cli_test.go`: Remove `TestCompressionMode`
- `openspec/specs/compression/spec.md`: Update requirements to reflect new
  configuration method
- `examples/routes/github-copilot.json`: Add commented compression example
