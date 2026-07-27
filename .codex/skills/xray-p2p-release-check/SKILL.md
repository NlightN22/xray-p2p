---
name: xray-p2p-release-check
description: Audit schema and persisted-data compatibility, verify a release candidate on automated gates and an authorized real-config canary, prepare and publish an xray-p2p release through the repository's GitHub Actions, and optionally bump the downstream xp2pdiag repository version without publishing its Docker image. Use when checking release readiness, comparing a release branch with its previous version, reviewing normalization or migration decisions, testing upgrades against real persisted state, updating generated TOML schemas and versions, reviewing a release diff, creating the release commit and annotated tag, running the confirmed external release workflow, or aligning xp2pdiag with a completed XP2P release.
---

# Xray P2P Release Check

Run each stage from the repository root and stop immediately on failure.

## Preflight

1. Run `python scripts/new_release.py check` before the final merge.
2. Report all working-tree changes shown by the command.
3. Run `make http-lifecycle-check` and require it to pass. Inspect all
   `nethttp-lifecycle:allow` directives changed since the previous release and
   stop on any exclusion that lacks a concrete owner, lifetime, and reason.
4. Run `make resource-plateau-nightly` and require the accelerated five-client
   control-plane plateau gate to pass for the exact release candidate SHA.
   Preserve and report the timestamped diagnostic log and workflow artifact.
5. Require schema drift checks, current schema fixtures, compatibility fixtures,
   the full Linux Go test view, and the full Linux host gate to pass.
6. Do not change the version, commit, tag, or push during this stage.

## Schema compatibility gate

Complete this gate after feature changes are assembled and before `prepare`.

1. Identify the previous release tag and the release-branch merge base. If the previous release has schemas, compare them with the generated schemas. Otherwise reconstruct the baseline from its accepted TOML, persisted models, loaders, and tests.
2. Review owner-package model and generated-schema changes. List removed, renamed, newly required, type-changed, enum-restricted, default-changed, and otherwise tightened fields. Include service-start transformations and credential, network, identity, or security behavior that changes persisted state.
3. Read `docs/en/flows/normalization-pipeline.md`. Classify every persisted change as additive, compatibility-normalized, explicitly migrated, or breaking. Do not treat `make schema-check` as compatibility evidence.
4. Add or update real legacy Desired inputs under `tests/schema/compat/<previous-version>/`. Run `make schema-compat` and require each fixture to pass both the current JSON Schema and its current owner-package runtime decoder/normalizer.
5. For every compatibility-normalized or migrated field, verify raw decode, compatibility rule, validation, canonical model, canonical write behavior, idempotence, and removal-version handling. Normal reads must not silently rewrite user-owned Desired files unless the documented service migration requires staging a change.
6. Produce a concise audit report containing the baseline, detected changes, compatibility evidence, normalization or migration behavior, user impact, required upgrade notes, and unresolved decisions.
7. Stop and discuss any unresolved migration, automatic rewrite, credential rotation, removal schedule, or breaking behavior with the user. Do not run `prepare` until the user accepts the decision or the implementation is changed.
8. Add user-visible effects and required actions to the release upgrade notes. Record a no-action result explicitly when all changes are additive or transparently normalized.

## Full Linux host gate

Complete this gate after the schema compatibility decisions are accepted and before `prepare`.

1. Start the Debian 12 test environment with `make up-deb12`. Use direct Vagrant commands only to diagnose a missing or failed Make target.
2. Run the complete Linux host suite as one pytest session with every opt-in release scenario enabled:

   ```powershell
   $env:XP2P_RUN_SERVICE_CLI_TESTS = '1'
   $env:XP2P_RUN_MANUAL_EDIT_TESTS = '1'
   $env:XP2P_RUN_HEARTBEAT_STORM_TESTS = '1'
   $env:XP2P_RUN_DESTRUCTIVE_TESTS = '1'
   $env:XP2P_RUN_DUAL_DEPLOY_TESTS = '1'
   $env:XP2P_RUN_EXTERNAL_SUBSCRIPTION_MATRIX = '1'
   New-Item -ItemType Directory -Force '.logs/tests' | Out-Null
   $logPath = ".\.logs\tests\pytest-linux-all-opt-in-{0}.log" -f (Get-Date -Format 'yyyyMMdd-HHmmss')
   pytest tests\host\linux -vv -s 2>&1 | Tee-Object -FilePath $logPath
   ```

3. Require a zero exit code. Report the passed, failed, error, skipped, and elapsed totals together with the log path.
4. Review every remaining skip. Accept only a documented dynamic skip caused by an unavailable guest prerequisite; an opt-in release scenario skipped because its environment variable was omitted fails the gate.
5. Treat targeted reruns as diagnostics only. After any test or infrastructure failure, restore the environment as needed and rerun the complete suite in one session before declaring the gate passed.
6. Stop release preparation on any failure, error, unexplained skip, guest provisioning problem, or cascade caused by missing test artifacts. Record the first independent failures separately from consequential failures.

## Prepare

1. Confirm the schema compatibility gate and full Linux host gate are complete, their decisions are accepted, all planned release changes are merged, and the working tree is clean.
2. Run `python scripts/new_release.py prepare --version X.Y.Z`.
3. Confirm the schema compatibility audit is complete and referenced in upgrade notes when user action is required.
4. Review the resulting diff. It must contain only the version file, OpenWrt package version, and generated schemas. It may be empty when the branch already contains the requested version.
5. Confirm schema drift, schema tests, schema compatibility, native Windows Go tests, WSL Go tests, and the full Linux host suite with all opt-in release scenarios passed.
6. Stop before commit, tag, or push so the user can inspect the release diff.

## Real-config canary gate

Complete this gate after `prepare` and before `publish`. It is a live-system
mutation and always requires the user's explicit approval of the target nodes,
maintenance window, candidate artifact, and rollback plan. Never infer access
or authorization from repository context.

1. Build the exact prepared release candidate locally. For OpenWrt, write IPKs
   under `build/ipk`; never use `openwrt/staging/stable`.
2. Select an authorized, representative working client/server pair with real
   accumulated Desired, Live, LKG, apply markers, credentials, and service
   enablement state. Prefer a pair that exercises TUN, redirects, control auth,
   heartbeat, and multiple endpoints when those features are release-relevant.
3. Before mutation, record package, binary, and Live runtime versions; enabled
   and running roles; process tree; Desired file hashes; the complete `.state`
   tree; tunnel/control ping; heartbeat freshness; and current service errors.
4. Back up `/etc/xp2p` and the package/runtime metadata to an access-restricted
   location outside the repository. Verify the archive can be listed. Never
   commit real configurations, credentials, certificates, unredacted state, or
   raw production logs.
   Retain every canary backup, rollback package, captured baseline, and
   diagnostic archive through the entire release process. Never delete these
   files during canary cleanup, failure investigation, rollback, successful
   publication, or final infrastructure cleanup. A completed or successful
   release is not authorization to remove them. Delete canary recovery data
   only as a separate operation after the user explicitly approves the exact
   paths; otherwise leave it on the protected target storage.
5. Install through the normal package upgrade path. Do not stop services,
   delete markers, rewrite credentials, or repair Desired/Live manually unless
   the documented package workflow requires it; the canary must test the same
   unattended migration users receive.
6. Test a mixed-version window: upgrade the server first and require the old
   client tunnel and control path to remain healthy. Then upgrade the client.
   If the supported rollout order differs, document and test that order before
   publication.
7. Require package, binary, and Live versions to match the candidate; no old
   processes to remain; pre-upgrade enabled/running state to be preserved;
   Desired hashes to remain unchanged except for an accepted migration; Live
   to contain current required metadata; and stale apply markers not to block
   compilation. Verify tunnel traffic, control ping/auth, heartbeat freshness,
   service restart persistence, logs, and the full `.state` tree.
8. Treat any manual recovery, unexpected credential change, stale Live,
   synthetic or stale heartbeat, process overlap, service-state drift, or
   unexplained marker as a failed gate. Capture a redacted logs-only diagnostic
   bundle, restore the previous package/state using the approved rollback, and
   verify recovery. Do not count a repaired canary as passed.
9. Convert every independent canary failure into an automated regression test,
   fix it, rerun all earlier affected gates, rebuild the candidate, and repeat
   the canary from the restored baseline.
10. Produce a concise canary report with target roles (not secrets or sensitive
    host identity), old/new versions, rollout order, backup location, checks,
    result, diagnostics location, rollback result when used, and unresolved
    risks. Record that the protected backups and recovery artifacts still
    exist, and do not mark the canary complete if they were removed before the
    investigation and release workflow ended. Block `publish`, tagging,
    pushing, and workflow dispatch until the canary passes.

## Publish locally

1. Proceed only after the user has reviewed the prepared diff and the
   real-config canary gate has passed.
2. Run `python scripts/new_release.py publish --version X.Y.Z`.
3. Confirm the release commit and local annotated `vX.Y.Z` tag were created. An empty release commit is expected when the version was already prepared.
4. Show `git status --short` and the tag target.

## External publication

Perform every push and workflow dispatch only after explicit user approval. Use the GitHub CLI in read-only mode while auditing or planning.

### Confirm the publication inputs

1. Confirm `gh auth status` succeeds and the account has `repo` and `workflow` access to `NlightN22/xray-p2p`.
2. Confirm the release branch, annotated `vX.Y.Z` tag, and release notes file identify the same release.
3. Push the release branch and tag only when approved. Require the automatic `ci` workflow to pass on the exact release commit SHA. `ci` is triggered by qualifying branch pushes and pull requests; do not dispatch it manually.
4. Confirm the `artifacts` branch contains the complete `openwrt/staging/stable/*.ipk` set for `X.Y.Z`. The GitHub build workflows do not create or publish this OpenWrt set.

### Keep local OpenWrt outputs out of the main worktree

1. Build local OpenWrt packages into `build/ipk`, using `scripts/build/build_openwrt_ipk.sh --all --force-build --output-dir build/ipk` or the equivalent guest path when the build runs inside Vagrant.
2. Never use `openwrt/staging/stable` as the local output directory in the main worktree. That path is reserved for the committed layout on the dedicated `artifacts` branch.
3. When publishing IPKs, copy only the verified versioned `*.ipk` files from `build/ipk` into `openwrt/staging/stable` inside a separate `artifacts`-branch worktree. Do not copy `Packages`, `Packages.gz`, release notes, logs, or other build outputs.
4. After publishing the artifacts branch, require the main worktree to contain no untracked or modified files under `openwrt/staging`. Move any accidental local outputs back to `build/ipk` and remove the empty staging directories before continuing.

### Build release artifacts

Run these three manual workflows for the same release ref. They are independent and may run in parallel:

- `build` (`build.yml`)
- `build-deb` (`build-deb.yml`)
- `build-msi` (`build-msi.yml`)

Set both the workflow dispatch ref and its `ref` input to the same release tag or branch. Prefer the pushed annotated tag so every run's `headSha` equals the tag commit. Require every run to succeed and verify its `headSha` before aggregation.

`Build MkDocs docs` (`build-mkdocs.yml`) is not an Aggregate Release dependency. It builds the docs artifact consumed later by Pages deployment. The Pages workflow deliberately downloads the latest successful MkDocs artifact from `main`, so ensure the intended documentation is on `main` and that its MkDocs run succeeded before deploying docs. A qualifying docs push to `main` starts it automatically; dispatch it manually only when a fresh artifact is required.

### Aggregate the GitHub release

Run `aggregate-release` (`aggregate-release.yml`) only after all of these are true:

- the annotated release tag exists on GitHub;
- `build`, `build-deb`, and `build-msi` succeeded for the exact tag commit;
- the complete versioned OpenWrt IPK set is present on `artifacts`;
- the release notes are final.

Use these inputs:

- `tag`: `vX.Y.Z`
- `include_openwrt_ipk`: `true`
- `artifacts_mode`: `sha`
- `artifacts_ref`: the release branch, used only as the workflow's fallback
- `release_notes_b64`: base64-encoded UTF-8 contents of the release notes file; leave `release_notes` empty when this is used

Reject an unexpected fallback from tag SHA to branch artifacts. After success, verify the versioned GitHub Release, its assets and `SHA256SUMS`, the forced `latest` tag, and the prerelease named `latest`.

### Deploy Pages

Run `Deploy GitHub Pages (feed + docs)` (`deploy-pages.yml`) after Aggregate Release succeeds:

- use `both` when publishing the current docs artifact and refreshing the OpenWrt feed;
- use `feed-only` when documentation must remain unchanged;
- use `docs-only` only when no release/feed update is intended.

The feed modes download IPKs from published stable GitHub Releases, so do not deploy the release feed before aggregation.

### Legacy workflow

The retired `release.yml` workflow was removed. Do not recreate or substitute a monolithic GitHub workflow for the confirmed build, aggregate, and Pages sequence.

Do not run `scripts/New-Release.ps1` in its current form. It implements the retired monolithic path: it permits non-`X.Y.Z` versions, can delete a remote tag before preflight, does not run the schema or full Linux gates, commits every tracked change with `git commit -am`, creates a lightweight tag, hard-codes pushing `main`, and treats `-Quiet` as approval for pushes. Redesign it before reuse so it delegates to `scripts/new_release.py`, keeps preparation separate from external publication, creates no unreviewed commits or tags, and requires explicit approval for every push and workflow dispatch.

## Optional downstream xp2pdiag version bump

Run this step only after the XP2P release is complete and the user explicitly
approves bumping the downstream repository. This step updates repository state;
that approval authorizes the scoped edits and a dedicated local commit, but it
does not authorize Git push or Docker image publication.

1. Use `E:\Programming\docker\xp2pdiag` unless the user provides another
   checkout. Confirm the target XP2P release exists and record its exact
   `X.Y.Z` version.
2. Inspect the xp2pdiag branch, working tree, remotes, and divergence. Preserve
   all existing local commits and user changes. Stop if unrelated uncommitted
   changes overlap the version bump.
3. Update the repository's version source and every matching version default or
   current-version example, including `version`, Dockerfile
   `XP2P_VERSION` defaults, and README version text. Keep the versioned download
   URL model and checksum verification intact.
4. Require all changed files to remain in English with LF line endings. Run the
   repository's lightweight static checks and `git diff --check`; do not invoke
   `build-push.ps1` without `-NoPush`.
5. Show the complete diff and status. Confirm that the diff contains only the
   intended xp2pdiag version bump and associated documentation.
6. Create a dedicated local version-bump commit and report its SHA. The approval
   to run this step does not authorize `git push`.
7. Never run `docker push`, publish or retag `latest`, or otherwise mutate
   Docker Hub in this step. Docker image build and publication are a separate
   downstream workflow requiring separate explicit approval.
8. Push the xp2pdiag Git branch only after a separate explicit approval, then
   report the pushed commit SHA. If no push is approved, leave the commit local
   and report the branch divergence.
