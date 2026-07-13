# Homebrew Distribution

## Current State

Releases are built via `.github/workflows/release.yml` using a matrix strategy
(5 targets) and uploaded via `softprops/action-gh-release`. No goreleaser is
used.

## Approach

A custom Homebrew tap (`koopycat/homebrew-copilot-monitor`) with a formula
auto-updated by the release workflow. A dedicated job in `release.yml` downloads
the release tarballs, computes SHA256s, and pushes the updated formula.

## Tap repo

The tap repository `github.com/koopycat/homebrew-copilot-monitor` must exist
before the first release that triggers the update job. Seed it with an initial
formula using real SHA256s from a published release.

### Initial formula template

```ruby
class CopilotMonitor < Formula
  desc "Local proxy for LLM API usage monitoring"
  homepage "https://github.com/koopycat/copilot-monitor"
  license "MIT"
  version "0.1.0"

  on_macos do
    on_arm do
      url "https://github.com/koopycat/copilot-monitor/releases/download/v0.1.0/copilot-monitor-darwin-arm64.tar.gz"
      sha256 "<REAL_SHA256>"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/koopycat/copilot-monitor/releases/download/v0.1.0/copilot-monitor-linux-arm64.tar.gz"
      sha256 "<REAL_SHA256>"
    end
    on_intel do
      url "https://github.com/koopycat/copilot-monitor/releases/download/v0.1.0/copilot-monitor-linux-amd64.tar.gz"
      sha256 "<REAL_SHA256>"
    end
  end

  def install
    bin.install "copilot-monitor"
  end

  test do
    system "#{bin}/copilot-monitor", "version"
  end
end
```

Replace `<REAL_SHA256>` with the actual hash from each tarball. On macOS use
`shasum -a 256 <file>`. On Linux use `sha256sum <file>`.

### Apple Silicon only

Intel macOS is intentionally excluded from the Homebrew formula. Intel Mac users
should use the direct download from GitHub Releases.

### Auto-update

The `homebrew` job in `.github/workflows/release.yml` runs after every tagged
release (`v*`). It downloads the new tarballs, computes SHA256s, replaces
placeholders in the formula template, validates the Ruby syntax, and pushes to
the tap repo.

## Secrets

The release workflow needs `HOMEBREW_TAP_TOKEN` — a GitHub fine-grained PAT with
`contents: write` on `koopycat/homebrew-copilot-monitor`. Set it in the main
repo's Actions secrets.

## Install

```sh
brew tap koopycat/copilot-monitor
brew trust koopycat/copilot-monitor
brew install copilot-monitor
```

## Future: homebrew-core

Once the project has sufficient adoption, submit the formula to
[homebrew-core](https://github.com/Homebrew/homebrew-core). The formula follows
homebrew-core conventions and can be submitted as-is.
