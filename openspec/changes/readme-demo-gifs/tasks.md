## 1. Verbose proxy demo tape

- [x] 1.1 Extend `demo/seed/main.go` to insert a few requests with timestamps
      clustered in the last hour (so the proxy's per-request log shows recent
      activity when the demo plays)
- [x] 1.2 Create `demo/copilot-monitor-nolive.tape` that starts the proxy with
      `--no-live`, shows the startup banner, then types a few CLI commands
      (`live`, `stats`) against the proxy while per-request log lines appear
- [x] 1.3 Add gifsicle post-processing step to the no-live GIF pipeline (same
      `--colors 32` as the overview GIF)

## 2. Justfile target

- [x] 2.1 Add `demo-gifs` target to `justfile` that runs seed → build → vhs for
      both tapes → gifsicle optimization

## 3. README integration

- [x] 3.1 Embed `demo/copilot-monitor.gif` in README under the "What it looks
      like" section (restore an animated demo section — currently was replaced
      with a text link)
- [x] 3.2 Embed `demo/copilot-monitor-nolive.gif` in README under the
      "Development" section, near the proxy startup instructions, with a caption
      explaining `--no-live` shows per-request logging
- [x] 3.3 Run `just demo-gifs` and verify both GIFs play correctly at their
      README display size

## 4. Validation

- [x] 4.1 Verify `just demo-gifs` completes without errors from clean state
- [x] 4.2 Verify both GIFs are under 600KB each (reasonable for GitHub README)
- [ ] 4.3 Open README on GitHub and confirm GIFs render inline with legible text
