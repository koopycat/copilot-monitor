# Pull Request Guidelines

## Commit messages

- Single line, no body, no bullet points, no multi-paragraph messages.
- Keep commits scoped. If the message needs a list or the word "and" multiple
  times to describe what changed, split the commit.

  **Good:** `Fix release workflow: pnpm/action-setup@v6 does not exist` **Bad:**
  `Fix GitHub workflow settings: pin action versions, align Node 24, add SARIF uploads and npm Dependabot entries`

## PR scope

A PR must be focused — it has a clear thesis. You can state in one sentence why
all the changes belong together. Multiple files and edits are fine as long as
they serve the same goal. If the PR body needs bullet-point sections for
unrelated things, it's too many PRs.

**Focused:** "Make security scanning results visible and reliable" — pins
trivy-action, adds SARIF uploads for both Trivy and OSV-Scanner. Two files, one
goal.

**Unfocused:** "Fix pnpm version, add SARIF, bump Node, add Dependabot entries"
— four unrelated concerns jammed into one PR.

## Before opening a PR

1. Run `git fetch origin && git diff origin/main` and verify every changed line
   is intentional. Stale local edits, merge artifacts, or accidentally staged
   files are your responsibility to catch.

2. When editing CI/CD files (workflows, Dependabot, pre-commit config), pull the
   latest main first. These files change frequently through automated version
   bumps and your edits may conflict or produce noisy diffs.

3. Check that the branch is based on an up-to-date main. Merge conflicts
   discovered after opening are a bad look and waste reviewer time.

## PR body

- Explain _why_ the change matters, not just _what_ changed. A reviewer should
  understand the motivation without reading the diff.
- Link to the analysis, spec, issue, or conversation that produced the change.
- Include test results: `just all` output, or explicitly note what was skipped
  and why.

  **Good:**

  > The release workflow references `pnpm/action-setup@v6`, which does not exist
  > (latest is v4). Every tagged release would crash before building the
  > dashboard. Fixes #12.
  >
  > Tested: `just all` passes. Release workflow validated by dry-run push.

  **Bad:**

  > Fixes several issues in the GitHub workflow and settings configuration.
  >
  > - release.yml: pnpm/action-setup@v6 → @v4
  > - security.yml: trivy-action@master → @0.29.0
  > - ci.yml: node-version 22 → 24
  > - dependabot.yml: Added npm entries

  The bad example describes _what_ but gives no reason for any change. It also
  combines four unrelated concerns in one PR.

## Merge preparation

- Prefer squash-merge for multi-commit PRs where the individual commits don't
  tell a useful story. Prefer rebase-merge when each commit is self-contained
  and reviewable on its own.
- Delete the branch after merge.
