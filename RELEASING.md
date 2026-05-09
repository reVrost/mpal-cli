# Releasing

Releases are handled by GoReleaser from version tags.

## One-time setup

The `Release` workflow needs a repository secret named
`HOMEBREW_TAP_GITHUB_TOKEN`. Use a GitHub token with write access to
`reVrost/homebrew-tap`.

## Cut a Release

```sh
git checkout main
git pull --ff-only
git tag v0.1.0
git push origin v0.1.0
```

Pushing the tag runs `.github/workflows/release.yml`, which:

- builds `mpal` and `mpal-mcp` for macOS and Linux
- creates a GitHub Release with checksums
- updates `reVrost/homebrew-tap` with a new `Formula/mpal.rb`

Users can then install or upgrade with:

```sh
brew install reVrost/tap/mpal
brew upgrade mpal
```
