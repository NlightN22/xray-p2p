# Release check

Use the release helper in three explicit stages:

1. Run `python scripts/new_release.py check` while changes are still being assembled. This reports working-tree changes, checks schema drift and fixtures, and runs the Linux Go test view without changing the version, committing, tagging, or pushing.
2. After all planned changes are merged and the working tree is clean, run `python scripts/new_release.py prepare --version X.Y.Z`. This updates release-owned files and schemas, runs the required checks, and stops with a reviewable diff.
3. After reviewing that diff, run `python scripts/new_release.py publish --version X.Y.Z`. This accepts only the prepared release-owned paths, creates the release commit and a local annotated tag, and does not push.

The remote push, release tag publication, and existing GitHub Actions order must be confirmed against a current release before it is encoded as an automated procedure. The helper intentionally leaves those external actions to a separately approved step.
