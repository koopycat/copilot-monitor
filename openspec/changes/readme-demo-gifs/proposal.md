## Why

The README is the first thing visitors see, but it's currently a wall of text.
Animated terminal demos (via vhs) give an immediate feel for the tool without
requiring a download or setup. The seed data and vhs tape infrastructure is
already in `demo/`; this extends it with a second demo showing verbose proxy
output (`--no-live`) and polishes both into the README.

## What Changes

- Add a second vhs tape `demo/copilot-monitor-nolive.tape` that shows the proxy
  running with `--no-live`, capturing per-request log output as typed commands
  hit the proxy
- Update `README.md` to embed both demo GIFs (overview + verbose proxy output)
  with brief captions
- Ensure the demo GIF pipeline is reproducible: run `just demo-gifs` to
  regenerate both

## Capabilities

### New Capabilities

- `readme-demos`: reproducible animated terminal demos for the README, generated
  via vhs from synthetic seed data

### Modified Capabilities

<!-- No existing spec requirements change. This is purely a README/docs change. -->

## Impact

- `demo/copilot-monitor.tape` (existing, possibly tweaked)
- `demo/copilot-monitor-nolive.tape` (new)
- `demo/seed/main.go` (existing, possibly extended with request data for the
  proxy-focused demo)
- `README.md` (embed GIFs)
- `justfile` (new `demo-gifs` target)
