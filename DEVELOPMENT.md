# Development

## Release

The `release` workflow runs [release-please](https://github.com/googleapis/release-please)
each time a pull request merges to `main`. Release-please reads the commit
messages since the last release. Write the commit messages in the Conventional
Commits format. Release-please selects the version number with these rules:

| commit message                                | version change |
| --------------------------------------------- | -------------- |
| `feat!:` or a `BREAKING CHANGE:` footer       | major          |
| `feat:`                                       | minor          |
| `fix:` or `perf:`                             | patch          |
| other types, such as `docs:` or `ci:`         | no release     |

Release-please opens a release pull request. The pull request updates
`CHANGELOG.md` with the new version. Merge the release pull request to make
the tag, such as `v1.2.3`, and the GitHub release.

When you merge the release pull request, the `binaries` job builds `forestry`
for macOS and Linux, on the amd64 and arm64 architectures. The job attaches
the binaries to the GitHub release. The job also sets the version in the
binaries, so that `forestry version` shows the tag.
