# Release Checklist

## One-time setup
- [ ] Add LICENSE
- [ ] Create GitHub repo
- [ ] Push `main`
- [ ] Update `.goreleaser.yml` placeholders
- [ ] Create optional Homebrew tap repo
- [ ] Add `HOMEBREW_TAP_GITHUB_TOKEN` secret if using Homebrew publishing

## Before each release
- [ ] `go test ./...`
- [ ] `go build ./cmd/devsweep`
- [ ] Review README
- [ ] Commit final changes
- [ ] Create tag: `git tag vX.Y.Z`
- [ ] Push tag: `git push origin vX.Y.Z`
- [ ] Verify GitHub Actions release passed
- [ ] Verify release artifacts and checksums exist

## After release
- [ ] Test install from GitHub release archive
- [ ] Test `devsweep version`
- [ ] If using Homebrew, test `brew install`
- [ ] Announce release
