## 1. Research and plan

- [x] 1.1 Verify goreleaser Homebrew support — current pipeline uses matrix
      build, goreleaser not needed; small bash step in release workflow is
      simpler
- [x] 1.2 Document the recommended approach in `docs/homebrew.md`

## 2. Tap repo setup

- [ ] 2.1 Create `homebrew-copilot-monitor` repository under koopycat org
      (requires GitHub UI)
- [ ] 2.2 Add initial formula file from `docs/homebrew.md` to the tap repo
- [ ] 2.3 Configure a GitHub token or deploy key for automated formula pushes
      from the release workflow

## 3. Release workflow update

- [x] 3.1 Add formula update step to `.github/workflows/release.yml` that
      computes SHA256s and pushes updated formula to the tap repo
- [ ] 3.2 Test with a pre-release tag to verify formula generation and push

## 4. README

- [x] 4.1 Add `brew install` section to README next to existing download table
- [ ] 4.2 Verify install works end-to-end on a clean macOS machine after first
      release with the tap
