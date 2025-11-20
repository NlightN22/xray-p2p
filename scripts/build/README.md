# Build Scripts

This directory hosts reproducible build helpers used by Vagrant guests, CI workflows, and local manual runs. Every script resolves paths relative to the repository root so they can be invoked either from the host (PowerShell/Bash) or from within the provisioned Vagrant boxes.

## Debian package (DEB)

Builds the `.deb` package with bundled Go binary and FPM metadata. Preconditions: run inside the Debian builder VM (`infra/vagrant/debian12/deb-build`) or on a Debian/Ubuntu host with go/fpm installed.

### Inside the Vagrant guest
```
cd /srv/xray-p2p
./scripts/build/build_deb_xp2p.sh
```
Artifacts land in `/srv/xray-p2p/build/deb/artifacts`. The same script is executed by the Vagrant provisioner (`infra/vagrant/debian12/deb-build/provision-deb-build.sh`).

### Directly on a Debian/Ubuntu host
```
./scripts/build/build_deb_xp2p.sh
```
Ensure the host has Go, FPM (`gem install fpm`), and the necessary build tools installed.
The resulting package installs `xp2p` under `/usr/bin` and drops shell completions into `/usr/share/bash-completion/completions/xp2p`, `/usr/share/zsh/vendor-completions/_xp2p`, and `/usr/share/fish/vendor_completions.d/xp2p.fish`.

## MSI (Windows installer)

Two PowerShell helpers exist: `build_and_install_msi.ps1` (amd64) and `build_and_install_msi_x86.ps1` (32-bit). Both compile xp2p.exe, copy `distro/windows/bundle/<arch>/xray.exe`, drive WiX (`installer/wix/xp2p*.wxs`), and optionally install the resulting MSI.

### Run on Windows host or Vagrant guest
```
# amd64
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build/build_and_install_msi.ps1

# x86
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build/build_and_install_msi_x86.ps1
```
The scripts default to `C:\xp2p` as the repo root/cache; override via `-RepoRoot`/`-CacheDir` parameters if needed. Additional parameters let you control the WiX source (`-WixSourceRelative`), the MSI name suffix (`-MsiArchLabel`), and whether the script should only build the MSI (`-BuildOnly`) instead of running `msiexec`. Use `-OutputMarker '__MSI_PATH__='` when another tool needs to parse the resulting path from `stdout`.

## OpenWrt SDK fetcher

`ensure_openwrt_sdk.sh` downloads (or refreshes) the OpenWrt SDK for selected targets and drops them into `~/openwrt-sdk-<identifier>`. Supported identifiers currently include `linux-amd64`, `linux-386`, `linux-arm64`, `linux-armhf`, and `linux-mipsle-softfloat`.

```
# grab every supported target (release matrix)
./scripts/build/ensure_openwrt_sdk.sh

# only refresh linux-amd64
./scripts/build/ensure_openwrt_sdk.sh linux-amd64
```

Override the defaults with `OPENWRT_VERSION`, `OPENWRT_MIRROR`, or `OPENWRT_SDK_BASE`.

## Bare xp2p binaries

`build_xp2p_binaries.sh` cross-compiles the CLI using Go's native toolchain and writes artefacts into `/tmp/build/<target>` (change via `XP2P_BUILD_ROOT`). Targets are mandatory (`--targets` / `--target` or `XP2P_TARGETS` env). By default binaries are stripped with `-s -w`, embed the `version.Current()` value, disable CGO (`CGO_ENABLED=0`), leave `GOEXPERIMENT` empty (override via `XP2P_GOEXPERIMENT`), run `strip --strip-unneeded`, copy the matching `distro/linux/bundle/<arch>/xray` into the target directory when available, and generate bash/zsh/fish completion scripts under `<target>/completions`.

```
# build linux-amd64 and linux-arm64
./scripts/build/build_xp2p_binaries.sh --targets "linux-amd64 linux-arm64"

# only linux-mipsle-softfloat into a custom folder
XP2P_TARGETS=linux-mipsle-softfloat XP2P_BUILD_ROOT=/tmp/xp2p \
  ./scripts/build/build_xp2p_binaries.sh
```

## OpenWrt ipk orchestrator

`build_openwrt_ipk.sh` automates the full OpenWrt pipeline: ensures the SDK exists, builds xp2p/xray/completions, installs the feed, applies diffconfig, compiles the ipk, and refreshes the local feed index. It reuses `ensure_openwrt_sdk.sh` and `build_xp2p_binaries.sh`, so all prerequisites for them apply (Go 1.21.7 toolchain, distro bundles, etc.). The canonical package recipe lives in `openwrt/feed/packages/utils/xp2p/Makefile`; the script leaves it in place and only references the feed when wiring the SDK. Release artefacts are copied into `openwrt/repo/<release>/<arch>/`, where `arch` is read from the resulting `.ipk` and `<release>` defaults to the value stored in `~/.xp2p-openwrt-version` (or `OPENWRT_VERSION` when set), so the repository doubles as the GitHub Pages feed. Pass `--output-dir build/ipk` (or any other folder) when you need the resulting `.ipk` and `Packages`/`Packages.gz` in a custom destination; this is what the OpenWrt Vagrant boxes use during provisioning.

```
# build linux-amd64 ipk and update openwrt/<release>/<arch>
./scripts/build/build_openwrt_ipk.sh \
  --target linux-amd64 \
  --sdk-dir ~/openwrt-sdk-linux-amd64 \
  --diffconfig openwrt/configs/diffconfig.linux-amd64 \
  --diffconfig-out openwrt/configs/diffconfig.linux-amd64

# build every supported target in one go (SDKs default to ~/openwrt-sdk-<target>)
./scripts/build/build_openwrt_ipk.sh --all

# write artefacts into build/ipk so infra/vagrant/openwrt can provision /tmp/build-openwrt
./scripts/build/build_openwrt_ipk.sh --target linux-amd64 --output-dir build/ipk
```

Omit `--diffconfig` if you want the SDK defaults; specify `--diffconfig-out` to capture the resulting configuration. The script stores artifacts under `/tmp/build/<target>` and copies the resulting `.ipk` into `openwrt/repo/<release>/<arch>`, regenerating `Packages`/`Packages.gz` (and `index.html` files) automatically. Run it once per target (e.g. `linux-armhf`, `linux-arm64`, `linux-mipsle-softfloat`, `linux-mips64le`, `linux-386`, `linux-amd64`); the underlying SDKs are cached and reused between invocations.

## Notes
- All scripts assume the repo root is mounted at either `/srv/xray-p2p` (guests) or the current working directory (host). Set `XP2P_PROJECT_ROOT` when you need to override detection.
- Windows PowerShell helpers rely on WiX Toolset being installed (e.g., `C:\Program Files (x86)\WiX Toolset v3.11`).
- For repeatable results, clean previous artifacts by removing `build/` or the per-script cache directories before running.
