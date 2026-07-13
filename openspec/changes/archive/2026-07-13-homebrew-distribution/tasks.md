## 1. Research and plan

- [x] 1.1 Verify goreleaser Homebrew support — current pipeline uses matrix
      build, goreleaser not needed; small bash step in release workflow is
      simpler
- [x] 1.2 Document the recommended approach in `docs/homebrew.md`

## 2. Tap repo setup

- [x] 2.1 Create `homebrew-copilot-monitor` repository under koopycat org
- [x] 2.2 Add initial formula file with real SHA256s from v0.1.0 release
- [x] 2.3 Configure `HOMEBREW_TAP_TOKEN` secret for automated formula pushes

## 3. Release workflow update

- [x] 3.1 Add formula update step to `.github/workflows/release.yml` that
      computes SHA256s and pushes updated formula to the tap repo
- [ ] 3.2 Test with a pre-release tag to verify formula generation and push

## 4. README

- [x] 4.1 Add `brew install` section to README next to existing download table
- [x] 4.2 End-to-end install verified on macOS: `brew tap`, `brew trust`,
      `brew install`, `copilot-monitor version` all succeed
