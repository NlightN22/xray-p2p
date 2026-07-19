---
name: xray-p2p-release-check
description: Audit schema and persisted-data compatibility, verify, prepare, and locally publish an xray-p2p release. Use when checking release readiness, comparing a release branch with its previous version, reviewing normalization or migration decisions, updating generated TOML schemas and versions, reviewing a release diff, or creating the release commit and local annotated tag.
---

# Xray P2P Release Check

Run each stage from the repository root and stop immediately on failure.

## Preflight

1. Run `python scripts/new_release.py check` before the final merge.
2. Report all working-tree changes shown by the command.
3. Require schema drift checks, current schema fixtures, compatibility fixtures, and the full Linux Go test view to pass.
4. Do not change the version, commit, tag, or push during this stage.

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

## Prepare

1. Confirm the schema compatibility gate is complete, its decisions are accepted, all planned release changes are merged, and the working tree is clean.
2. Run `python scripts/new_release.py prepare --version X.Y.Z`.
3. Confirm the schema compatibility audit is complete and referenced in upgrade notes when user action is required.
4. Review the resulting diff. It must contain only the version file, OpenWrt package version, and generated schemas. It may be empty when the branch already contains the requested version.
5. Confirm schema drift, schema tests, schema compatibility, native Windows Go tests, and WSL Go tests passed.
6. Stop before commit, tag, or push so the user can inspect the release diff.

## Publish locally

1. Proceed only after the user has reviewed the prepared diff.
2. Run `python scripts/new_release.py publish --version X.Y.Z`.
3. Confirm the release commit and local annotated `vX.Y.Z` tag were created. An empty release commit is expected when the version was already prepared.
4. Show `git status --short` and the tag target.

## External publication

Do not push the branch or tag and do not trigger GitHub Actions automatically. Remind the user that the exact order of branch push, tag publication, and existing GitHub Actions must be confirmed against a current release and separately approved. After that process is verified, update this skill with the confirmed sequence.
