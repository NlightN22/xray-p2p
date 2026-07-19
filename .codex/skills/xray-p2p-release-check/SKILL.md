---
name: xray-p2p-release-check
description: Verify, prepare, and locally publish an xray-p2p release. Use when checking release readiness, updating the project version and generated TOML schemas, reviewing a release diff, or creating the release commit and local annotated tag.
---

# Xray P2P Release Check

Run each stage from the repository root and stop immediately on failure.

## Preflight

1. Run `python scripts/new_release.py check` before the final merge.
2. Report all working-tree changes shown by the command.
3. Require schema drift checks, schema fixtures, and the full Linux Go test view to pass.
4. Do not change the version, commit, tag, or push during this stage.

## Prepare

1. Confirm all planned release changes are merged and the working tree is clean.
2. Run `python scripts/new_release.py prepare --version X.Y.Z`.
3. Review the resulting diff. It must contain only the version file, OpenWrt package version, and generated schemas. It may be empty when the branch already contains the requested version.
4. Confirm both native Windows and WSL Go checks passed.
5. Stop before commit, tag, or push so the user can inspect the release diff.

## Publish locally

1. Proceed only after the user has reviewed the prepared diff.
2. Run `python scripts/new_release.py publish --version X.Y.Z`.
3. Confirm the release commit and local annotated `vX.Y.Z` tag were created. An empty release commit is expected when the version was already prepared.
4. Show `git status --short` and the tag target.

## External publication

Do not push the branch or tag and do not trigger GitHub Actions automatically. Remind the user that the exact order of branch push, tag publication, and existing GitHub Actions must be confirmed against a current release and separately approved. After that process is verified, update this skill with the confirmed sequence.
