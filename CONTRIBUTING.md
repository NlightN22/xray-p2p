# Contributing

Thanks for helping improve XRAY-p2p! This document focuses on developer tasks for the Go CLI located under `./go`.

## Local development

- Install Go (version is defined via `go.mod`).
- Run `go mod tidy` only if you intentionally manage dependencies.
- Use `go fmt ./...` before sending changes; CI will double-check formatting.
- Build binaries with `make build`. Supported targets are defined once in `go/internal/buildtarget`, and the helper `go/tools/targets` drives both the Makefile and CI so packages stay in sync. The version is injected via ldflags and binaries keep their platform-specific names (`xp2p`, `xp2p.exe`).
- CLI commands are configuration-only: they must update TOML/JSON state and routing files but never touch OS networking directly. OS-level changes (TUN, routes, nftables) are applied only by `xp2p {client|server} run` or the system service.
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
- When adding or modifying tests, use the shared failure dump helpers (for example `tests/host/openwrt/_helpers.dump_failure_state`) instead of ad-hoc diagnostic dumps.

## Versioning and releases

- Check the CLI version with `xp2p --version`. On startup the binary logs the embedded version, and deployment commands include it in their output.
- The canonical version string lives in `go/internal/version/version.go`. Update `current` before releasing so `go run ./go/cmd/xp2p --version` reports the target number.
- CI builds embed the version via `-ldflags "-X .../version.current=$VERSION"` and package archives named `xp2p-<version>-<os>-<arch>`.
- Release flow:
  1. Run `go test ./...` and `go vet ./...`.
  2. Commit the version bump and related changes.
  3. Tag the commit (`git tag vX.Y.Z && git push origin vX.Y.Z`).
  4. The `release` workflow reads the same target catalog and rebuilds binaries with the tag version, publishes archives `xp2p-<version>-<os>-<arch>`, force-updates the `latest` tag, and republishes `xp2p-latest-<os>-<arch>` assets for stable download links.
  5. You can run `scripts/New-Release.ps1 -Version X.Y.Z` to update `go/internal/version/version.go`, verify tests/builds, and get the exact commit/tag commands before tagging.

## Continuous integration

- `ci.yml`: gofmt check, `go vet`, unit tests, and integration tests.
- `build.yml`: cross-platform build matrix and smoke test. Outputs match the release artifact naming.
- `release.yml`: runs on tags `v*`, verifies sources, builds archives, and publishes the GitHub release.

Please open issues for major changes before starting implementation. Pull requests should describe the motivation, highlight risky areas, and include testing notes (commands run, results, environment). Thank you!
