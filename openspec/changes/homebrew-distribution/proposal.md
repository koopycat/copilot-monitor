## Why

The project currently distributes binaries via GitHub Releases with a download
table in the README. Adding a Homebrew formula would give macOS users a
one-command install (`brew install copilot-monitor`) and automatic updates via
`brew upgrade`. This is the standard distribution channel for macOS CLI tools
and signals maturity.

This change is an analysis — research the options, pick the best approach, and
document the plan. Implementation follows in a separate change.

## What Changes

- Research how Go CLI tools distribute via Homebrew (goreleaser, custom formula,
  homebrew-core vs custom tap)
- Evaluate trade-offs: homebrew-core inclusion vs custom tap, goreleaser
  automation vs manual formula, binary bottles vs source builds
- Assess what our release pipeline needs to change (new GitHub Actions steps,
  formula template, version bumping)
- Document the recommended approach and create an implementation plan

## Capabilities

### New Capabilities

- `homebrew-distribution`: analysis and plan for distributing copilot-monitor
  via Homebrew

### Modified Capabilities

<!-- Analysis only, no spec changes. -->

## Impact

- `docs/` — new documentation on Homebrew distribution approach
- `.github/workflows/` — potential new CI steps identified (not implemented)
- `README.md` — `brew install` badge/instructions (future)
