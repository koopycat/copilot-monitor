## Context

The project already has `demo/seed/main.go` for synthetic data generation and
`demo/copilot-monitor.tape` for a vhs-driven CLI overview GIF. This change adds
a second tape showing verbose proxy output (`--no-live`) and integrates both
GIFs into the README.

## Goals / Non-Goals

**Goals:**

- Add a reproducible `copilot-monitor-nolive.tape` showing the proxy running
  with `--no-live`, where per-request log lines appear as commands are typed
- Embed both demo GIFs in README.md with brief captions
- Add a `just demo-gifs` target to regenerate both GIFs in one command

**Non-Goals:**

- Recording real traffic. Demos use `demo/seed/` synthetic data only.
- Automating GIF generation in CI (vhs requires ttyd + ffmpeg, too heavy).
- Dashboard screenshots. vhs captures terminal output only.

## Decisions

1. **Separate tape file per demo** rather than one mega-tape. Each GIF tells a
   focused story: one shows the reporting commands, the other shows the proxy's
   per-request log output.

2. **`just demo-gifs` target** orchestrates seed → build → vhs → gifsicle for
   both tapes. Keeps the multi-step workflow as a single command.

3. **GIFs stored in `demo/`** not in a separate assets branch. They're under 1MB
   combined and don't change frequently enough to warrant branch management.

4. **gifsicle post-processing** with `--colors 32` keeps file sizes small
   without sacrificing readability of terminal text.

## Risks / Trade-offs

- [GIFs go stale] → They're generated from synthetic seed data, so
  `just demo-gifs` always produces the same output. If CLI output format
  changes, re-run to update.
