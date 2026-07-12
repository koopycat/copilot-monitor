## Why

The old `--dashboard` combined proxy and dashboard on the same port using
`combinedDashProxy` + `acceptsHTML` header sniffing. This was fragile and caused
API requests to get dashboard 404s. Rather than fixing the multiplexing, run the
dashboard on its own port (7734) as a sidecar goroutine inside the `run`
process. Same convenience ("one command"), no fragility.

## What Changes

- Add `--dashboard` flag back to `run`, but instead of combining into one mux,
  it starts a second HTTP listener on port 7734 serving the dashboard API + UI
- Dashboard server runs in a goroutine, shares the same `store.Store` with the
  proxy
- Graceful shutdown: both listeners stop on SIGINT/SIGTERM
- `copilot-monitor serve` still works as a standalone dashboard process

## Capabilities

### Modified Capabilities

- `dashboard`: Combined single-port mode is replaced by separate-port sidecar
  mode. Dashboard starts on its own port in the `run` process.

## Impact

- `internal/cli/run.go`: Add `--dashboard` flag, start dashboard goroutine
- `internal/cli/completion.go`: Add `--dashboard` back
- README: Update flag table and examples
