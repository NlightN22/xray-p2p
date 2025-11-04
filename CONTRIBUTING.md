# Contributing

Thanks for helping improve XRAY-p2p! This document focuses on developer tasks for the Go CLI located under `./go`.

## Local development

- Install Go (version is defined via `go.mod`).
- Run `go mod tidy` only if you intentionally manage dependencies.
- Use `go fmt ./...` before sending changes; CI will double-check formatting.
- Build binaries with `make build`. The Makefile sets `GOOS`/`GOARCH` combinations and injects the version via ldflags while keeping binary names (`xp2p`, `xp2p.exe`).

## Testing

- Unit tests: `go test ./...`
- Integration suite (requires additional dependencies): `go test -tags=integration ./...`
- Windows smoke workflows are described in `tests/README.md`. They run automatically in CI when triggered.

## Versioning and releases

- Check the CLI version with `xp2p --version`. On startup the binary logs the embedded version, and deployment commands include it in their output.
- The canonical version string lives in `go/internal/version/version.go`. Update `current` before releasing so `go run ./go/cmd/xp2p --version` reports the target number.
- CI builds embed the version via `-ldflags "-X .../version.current=$VERSION"` and package archives named `xp2p-<version>-<os>-<arch>`.
- Release flow:
  1. Run `go test ./...` and `go vet ./...`.
  2. Commit the version bump and related changes.
  3. Tag the commit (`git tag vX.Y.Z && git push origin vX.Y.Z`).
  4. The `release` workflow rebuilds binaries with the tag version, publishes archives `xp2p-<version>-<os>-<arch>`, force-updates the `latest` tag, and republishes `xp2p-latest-<os>-<arch>` assets for stable download links.

## Continuous integration

- `ci.yml`: gofmt check, `go vet`, unit tests, and integration tests.
- `build.yml`: cross-platform build matrix and smoke test. Outputs match the release artifact naming.
- `release.yml`: runs on tags `v*`, verifies sources, builds archives, and publishes the GitHub release.

Please open issues for major changes before starting implementation. Pull requests should describe the motivation, highlight risky areas, and include testing notes (commands run, results, environment). Thank you!
