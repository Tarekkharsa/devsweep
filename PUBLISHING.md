# Publishing DevSweep

This repo is set up for:
- CI on every push / pull request
- tagged GitHub releases using GoReleaser
- optional Homebrew publishing through a separate tap repo

## 1. Before publishing

### Choose a license
Add a license before making the repo public.

Recommended simple choice:
- MIT

If you use GitHub's UI:
- go to **Add file → Create new file**
- click **Choose a license template**
- pick **MIT License**

### Make README install instructions release-friendly
Once your GitHub repo exists, update links and install examples to point at the real repo URL.

### Check Go version compatibility
Your `go.mod` currently controls the Go version used by CI and releases.

## 2. Create the GitHub repo

Example with GitHub CLI:

```bash
gh repo create devsweep --public --source=. --remote=origin --push
```

Or create it in the GitHub web UI, then add the remote manually:

```bash
git remote add origin git@github.com:YOUR_GITHUB_USERNAME/devsweep.git
git push -u origin main
```

## 3. Enable CI

CI is already configured in:
- `.github/workflows/ci.yml`

After pushing to GitHub, Actions will run automatically.

## 4. Configure release publishing

Tagged releases are configured in:
- `.github/workflows/release.yml`
- `.goreleaser.yml`

### First-time setup
Edit `.goreleaser.yml` and replace:
- `YOUR_GITHUB_USERNAME`
- the Homebrew tap repo name if needed

Also update the homepage URL there.

## 5. Create a GitHub release

When you want to publish a version:

```bash
git tag v0.1.0
git push origin v0.1.0
```

That will trigger the release workflow and publish:
- macOS amd64
- macOS arm64
- Linux amd64
- Linux arm64
- checksums

## 6. Optional: publish to Homebrew

If you want `brew install devsweep`, create a separate tap repo.

Example:

```bash
gh repo create homebrew-tap --public
```

GoReleaser expects a repo like:
- `YOUR_GITHUB_USERNAME/homebrew-tap`

### Required secret
Create a classic GitHub token or fine-grained token with access to the tap repo, then add it as a repo secret in the main repo:

- Name: `HOMEBREW_TAP_GITHUB_TOKEN`

GitHub UI path:
- **Repo → Settings → Secrets and variables → Actions → New repository secret**

Once that is set, future tagged releases can update the Homebrew formula automatically.

## 7. Recommended release checklist

Before tagging:

```bash
go test ./...
go build ./cmd/devsweep
./devsweep version
./devsweep scan --help 2>/dev/null || true
```

Then:

1. update README if needed
2. commit changes
3. tag the release
4. push the tag
5. verify GitHub Actions succeeded
6. verify artifacts exist in GitHub Releases
7. test install from a release tarball

## 8. Suggested first public release flow

```bash
git checkout main
git pull --ff-only
go test ./...
go build ./cmd/devsweep
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

## 9. If you want Homebrew users to install it

After Homebrew tap publishing works, users can install with:

```bash
brew tap YOUR_GITHUB_USERNAME/tap
brew install devsweep
```

Or if you keep the repo named `homebrew-tap`, the usual pattern is:

```bash
brew tap YOUR_GITHUB_USERNAME/tap
```

If you prefer a cleaner tap name, rename the tap repo to:
- `homebrew-tap`
- or `homebrew-tools`

Then update `.goreleaser.yml` accordingly.

## 10. Nice follow-ups after publishing

- add a LICENSE file
- add screenshots / terminal examples to README
- add a short roadmap section
- add shell completions later if users ask for them
