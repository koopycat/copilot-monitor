<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Homebrew distribution via custom tap

The project SHALL be installable via Homebrew using a custom tap formula that
points at GitHub Release binaries. The formula SHALL be auto-updated by the
release workflow on each tagged release.

#### Scenario: User installs via Homebrew

- **WHEN** a macOS user runs
  `brew tap koopycat/copilot-monitor && brew install copilot-monitor`
- **THEN** the latest release binary is downloaded and installed to
  `/opt/homebrew/bin/copilot-monitor` (Apple Silicon)

#### Scenario: Formula updates on release

- **WHEN** a new GitHub Release is published via the release workflow
- **THEN** the formula in the `homebrew-copilot-monitor` tap repo is
  automatically updated with the new version and SHA256

### Requirement: Release workflow formula update

The release workflow SHALL include a `homebrew` job that downloads release
tarballs, computes SHA256s, generates an updated formula, validates Ruby syntax,
and pushes to the tap repository.

#### Scenario: Workflow updates formula

- **WHEN** the release workflow runs on a `v*` tag
- **THEN** the `homebrew` job clones the tap repo, generates a formula with
  correct version and SHA256s for `darwin-arm64`, `linux-arm64`, and
  `linux-amd64`, and pushes the updated formula
- **AND** the formula passes `ruby -c` syntax validation before push

### Requirement: README documents Homebrew install

The README SHALL include `brew install` instructions alongside the existing
download table.

#### Scenario: README shows brew install

- **WHEN** a visitor reads the README
- **THEN** they see
  `brew tap koopycat/copilot-monitor && brew install copilot-monitor` as an
  install method
