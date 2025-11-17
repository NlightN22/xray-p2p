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

## OpenWrt ipk

`build_openwrt_xp2p.sh` orchestrates SDK downloads, repo syncing, feed wiring, and ipk builds.

### Inside the Debian OpenWrt builder VM
```
cd /srv/xray-p2p
./scripts/build/build_openwrt_xp2p.sh
```
Use `XP2P_TARGETS`, `XP2P_TARGETS="linux-arm64"`, `XP2P_KEEP_CONFIG=1`, etc., to control targets. Artifacts are copied to `/srv/xray-p2p/build/openwrt/<identifier>`.

### CI usage
GitHub workflows call the same script via `$GITHUB_WORKSPACE/scripts/build/build_openwrt_xp2p.sh`, exporting `XP2P_PROJECT_ROOT` and `XP2P_BUILD_ROOT` as needed.

## Notes
- All scripts assume the repo root is mounted at either `/srv/xray-p2p` (guests) or the current working directory (host). Set `XP2P_PROJECT_ROOT` when you need to override detection.
- Windows PowerShell helpers rely on WiX Toolset being installed (e.g., `C:\Program Files (x86)\WiX Toolset v3.11`).
- For repeatable results, clean previous artifacts by removing `build/` or the per-script cache directories before running.
