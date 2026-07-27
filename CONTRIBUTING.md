# Contributing

Thanks for helping improve XRAY-p2p! This document focuses on developer tasks for the Go CLI located under `./go`.

## Local development

- Install Go (version is defined via `go.mod`).
- Run `go mod tidy` only if you intentionally manage dependencies.
- Use `go fmt ./...` before sending changes; CI will double-check formatting.
- Build binaries with `make build`. Supported targets are defined once in `go/internal/buildtarget`, and the helper `go/tools/targets` drives both the Makefile and CI so packages stay in sync. The version is injected via ldflags and binaries keep their platform-specific names (`xp2p`, `xp2p.exe`).
- CLI commands are configuration-only: they must update Desired inputs under `CONFIG_ROOT` (TOML and optional extension JSON snippets) and apply marker files, but never edit compiled runtime artifacts under `.state/`. OS-level changes (TUN, routes, nftables) are applied only by `xp2p {client|server} run` or the system service.
- When authoring service units or packaging hooks (systemd or procd), run `xp2p ... service run` without extra CLI flags. Services must rely on the default configuration baked into the binary, and packages must manage enabling/disabling them without injecting custom flags so upgrades do not require flag migrations.
- Logs are stored under `/var/log/xp2p` by default on Linux/OpenWrt; set `XP2P_LOG_ROOT` to override. The audit log lives at `<log root>/audit.log`.
- Windows Vagrant guests ship with evaluation licenses. Once the license expires `wlms.exe` will power off the VM every few hours (Event ID 1074/User32). Refresh or re-arm the license before running Windows host tests to avoid silent shutdowns during pytest.
- Windows runs of xray-core may log `Failed to find matching adapter name` and `Removed orphaned adapter` during Wintun adapter setup; treat these as expected startup noise unless service functionality is impacted.

## Routing rules

- Client routing rules are ordered as: endpoint bypass (direct) first, then other system rules, then redirects, and finally user-defined rules.
- Endpoint bypass rules must match each endpoint address and use the direct outbound tag (`direct` on Unix-like platforms, `direct-random` on Windows).
- System rules include reverse bridge rules, diagnostics marker rules, endpoint routing rules, and Windows direct fallback rules.
- Client outbounds must store resolved IP addresses for endpoint servers (never raw domain names), regardless of tun mode. Keep SNI/ALPN in stream settings unchanged.

## Testing

- Unit tests: `go test ./...`
- Integration suite (requires additional dependencies): `go test -tags=integration ./...`
- Windows smoke workflows are described in `tests/README.md`. They run automatically in CI when triggered.
- Deployment packages: `go test ./go/internal/deploy` checks the embedded templates and archive layout.
- When adding or modifying tests, use the shared failure dump helpers (for example `tests/host/openwrt/_helpers.dump_failure_state` or `tests/host/win/env.dump_failure_state`) instead of ad-hoc diagnostic dumps.

### Network lifecycle review

For changes to clients, servers, endpoints, background network flows, retries, or shutdown:

- Where is each network resource created, and who owns it?
- When and how is it closed, including timeout, cancellation, and recovery paths?
- Is it reused between iterations, with stale resources pruned?
- Does every response path bound reads or drains and close the body?
- Does a periodic flow include resource plateau evidence?
- Do server-side idle limits contain faulty or disconnected peers?

Run `make http-lifecycle-check` and the focused lifecycle tests for the affected package. Run `make resource-plateau` when a periodic control-plane flow changes.

## Persisted data compatibility

Use the normalization pipeline for xp2p-owned persisted data that can evolve over time, including TOML configuration, JSON state files, and future xp2p-owned metadata. Do not use it for Xray JSON because that is an external runtime format.

The standard flow is:

```text
Raw decode -> Defaults -> Compatibility rules -> Validation -> Canonical model -> Optional canonical write
```

Keep domain logic in the domain package. The shared `go/internal/normalize` package must contain only generic primitives (`Report`, `Rule`, `Pipeline`) and must not know concrete fields from config or state files.

When adding or changing a persisted field:

- Add raw and canonical models in the domain package when the current file shape has legacy syntax.
- Keep old fields in raw models only.
- Add one explicit compatibility rule per legacy syntax.
- Set `Name`, `Description`, `DeprecatedSince`, `RemovedSince`, and `RemovalNote` on every rule.
- Reject the legacy syntax when `version.Current()` is greater than or equal to `RemovedSince`.
- Do not add `schema_version` fields or version-by-version migration chains for user TOML, JSON state, or metadata files.
- Do not rewrite files on normal reads.
- Write canonical format on the next normal save path for that file.

Each domain pipeline must have tests for canonical input, legacy input, mixed input with the same meaning, conflicting legacy/canonical fields, defaults, invalid values, and canonical writes that omit deprecated fields.

## Versioning and releases

- Check the CLI version with `xp2p --version`. On startup the binary logs the embedded version, and deployment commands include it in their output.
- The canonical version string lives in `go/internal/version/version.go`. Update `current` before releasing so `go run ./go/cmd/xp2p --version` reports the target number.
- CI builds embed the version via `-ldflags "-X .../version.current=$VERSION"` and package archives named `xp2p-<version>-<os>-<arch>`.
- Release flow:
  1. Complete the schema compatibility audit and full opt-in Linux host gate.
  2. Run `python scripts/new_release.py prepare --version X.Y.Z`, review the generated release diff, then run `python scripts/new_release.py publish --version X.Y.Z` to create the local release commit and annotated tag.
  3. Push the release branch and tag, then require the automatic `ci` run to pass on the release commit.
  4. Publish the complete versioned OpenWrt `.ipk` set to the `artifacts` branch.
  5. Run `build.yml`, `build-deb.yml`, and `build-msi.yml` for the exact release tag.
  6. Run `aggregate-release.yml` with the release notes after all artifacts succeed.
  7. Run `deploy-pages.yml` after aggregation to update the OpenWrt feed and, when needed, documentation.
- `scripts/New-Release.ps1` implements the retired monolithic release path and must not be used until it is redesigned around the staged release flow.

## Continuous integration

- `ci.yml`: validates pinned Xray assets, lints generated schemas, and runs Go tests on qualifying pushes and pull requests.
- `build.yml`: manually builds the cross-platform archive matrix for a selected ref.
- `build-deb.yml` and `build-msi.yml`: manually build release installers for a selected ref.
- `build-mkdocs.yml`: builds documentation on qualifying `main` pushes or manual dispatch.
- `aggregate-release.yml`: assembles successful build artifacts and publishes versioned and `latest` GitHub Releases.
- `deploy-pages.yml`: publishes the OpenWrt feed and documentation to GitHub Pages.

Please open issues for major changes before starting implementation. Pull requests should describe the motivation, highlight risky areas, and include testing notes (commands run, results, environment). Thank you!
