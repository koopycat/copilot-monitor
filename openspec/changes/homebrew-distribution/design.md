## Context

copilot-monitor is a single Go binary distributed via GitHub Releases. macOS is
the primary target audience. Homebrew is the standard package manager for macOS
CLI tools. This design evaluates how to add Homebrew distribution.

## Goals / Non-Goals

**Goals:**

- Evaluate Homebrew distribution options for a Go CLI tool
- Recommend the simplest approach that works with the existing GitHub Releases
  pipeline (matrix build + `softprops/action-gh-release`, no goreleaser)
- Document what CI changes are needed

**Non-Goals:**

- Implementing the formula (separate change)
- Linux package managers (apt, rpm, nix) -- out of scope for this analysis

## Decisions

### Option A: Custom tap + bash workflow step (recommended)

Create a `homebrew-copilot-monitor` tap repo with a formula auto-updated by a
dedicated job in the release workflow.

**How it works:**

1. Matrix build produces tarballs, uploads to GitHub Release
2. A `homebrew` job runs after the build, downloads the tarballs, computes
   SHA256s, generates a Ruby formula from a heredoc template, validates syntax,
   and pushes to the tap repo
3. User runs `brew tap koopycat/copilot-monitor && brew install copilot-monitor`

**Pros:**

- No new dependencies — uses existing `curl`, `sha256sum`, `ruby`, `git`
- Full control over formula structure
- No goreleaser migration needed (the current build matrix + dashboard build
  would be complex to port)
- Can ship immediately after tap repo is created

**Cons:**

- Two-step install (`brew tap` + `brew install`) vs single command
- Need to maintain a separate repo for the formula
- Formula template is hand-maintained in the workflow (not auto-generated)

### Option B: homebrew-core inclusion

Submit the formula to homebrew-core, the main Homebrew repository.

**Pros:**

- Single command install: `brew install copilot-monitor`
- No separate tap repo to maintain

**Cons:**

- Strict acceptance criteria (notable usage, maintained project)
- Review process can take weeks or be rejected
- Not guaranteed to be accepted for a new project

### Recommendation: Option A then Option B

Start with Option A (custom tap + bash workflow step) for immediate
availability, then pursue Option B (homebrew-core) once the project has enough
adoption.

## Implementation Plan

1. Create `homebrew-copilot-monitor` repo under the `koopycat` org
2. Seed with initial formula using real SHA256s from a published release
3. Add `HOMEBREW_TAP_TOKEN` secret to the main repo
4. Add `homebrew` job to `.github/workflows/release.yml`
5. Update README with `brew install` instructions
6. (Later) Submit to homebrew-core

## Risks / Trade-offs

- [Formula template drifts from actual release] → The heredoc in the workflow is
  the single source of truth; any structure change must update both the workflow
  and the tap repo's initial formula
- [homebrew-core rejection] → Custom tap is the fallback, users still get a
  one-line install after `brew tap`
