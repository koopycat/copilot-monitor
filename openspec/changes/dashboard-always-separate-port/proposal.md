## Why

`copilot-monitor run --dashboard` served the proxy and dashboard on the same
port, requiring a `combinedDashProxy` mux with Accept-header sniffing to
distinguish API from browser traffic. This was fragile — we just fixed a bug
where API requests got dashboard 404s. The dashboard should run on its own port
always, just like `copilot-monitor serve` does. No combined mode, no header
sniffing, no ambiguity.

## What Changes

- **BREAKING**: Remove `--dashboard` flag from `copilot-monitor run`
- Remove `combinedDashProxy` and `acceptsHTML` from `run.go`
- Remove `dashboard` import from `run.go`
- Dashboard is always served via `copilot-monitor serve` on port 7734
- Update completion entries

## Capabilities

### Modified Capabilities

- `dashboard`: Combined mode removed. Dashboard always on separate port via
  `serve`.

## Impact

- `internal/cli/run.go`: Remove `--dashboard` flag, `combinedDashProxy`,
  `acceptsHTML`, dashboard import
- `internal/cli/completion.go`: Remove `--dashboard` completion
- `openspec/specs/dashboard/spec.md`: Remove browser-only routing requirement
  (no longer needed)
- README and docs: Update examples that use `--dashboard`
