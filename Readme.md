# XRAY-p2p (Go)

XRAY-p2p delivers a cross-platform Trojan tunnel built on top of `xray-core`. The Go-based `xp2p` CLI owns server and client provisioning, state tracking, TLS assets, and helper routes on Windows, Linux, and OpenWrt so you no longer need to depend on the original shell automation.

> Need the legacy shell workflow? The archived text lives in [Readme-old.md](Readme-old.md).

## What xp2p provides

- A single statically linked CLI (`xp2p`) with Cobra-based help, completions, doc generation, and a background diagnostics service.
- Server management covering installation, upgrades, TLS deployment, user provisioning, redirect/forward/reverse bridges, and `xray-core` log collection.
- Client management on Windows, Linux, and OpenWrt including endpoint installs from `trojan://` links, reverse portals, SOCKS autodiscovery, redirects, and DNS-aware forwarding.
- Remote deployment handshakes (`xp2p client deploy` + `xp2p server deploy`) that ship ready-to-use manifests over an encrypted link before bootstrapping both sides.
- Build tooling that emits per-OS packages together with vendor-supplied `xray` binaries, Windows MSI installers, Debian packages, and OpenWrt IPKs.

## Getting xp2p

### Download release archives

Grab pre-built archives from the GitHub Releases page. File names follow `xp2p-<version>-<target>.zip` on Windows and `xp2p-<version>-<target>.tar.gz` on Linux. Each archive contains the `xp2p` binary and the matching `xray` binary staged under the same directory, so unpack it anywhere and add it to `PATH` (or point services at that folder).

Release targets:

- `windows-amd64` (`.zip`)
- `windows-386` (`.zip`)
- `linux-amd64` (`.tar.gz`)
- `linux-386` (`.tar.gz`)
- `linux-arm64` (`.tar.gz`)
- `linux-armhf` (`.tar.gz`)

Additional experimental targets (MIPS softfloat, MIPS64LE, RISC-V) are available via the build tooling but are not uploaded automatically.

Use the helper tool to inspect identifiers and asset names:

```bash
go run ./go/tools/targets list --scope release
go run ./go/tools/targets assets --version 0.1.5 --name xp2p
```

### Install from source

1. Install Go 1.21.7+ (the repo sets `toolchain go1.21.7` in `go.mod` to ensure reproducibility).
2. Clone this repository and install the CLI straight from source:
   ```bash
   git clone https://github.com/NlightN22/xray-p2p.git
   cd xray-p2p
   go install ./go/cmd/xp2p@latest
   xp2p --help
   ```
3. Cross-compile or stage binaries with the provided helpers:
   ```bash
   make build                                    # builds every release target into build/<target>
   go run ./go/tools/targets build --target linux-arm64
   go run ./go/tools/targets deps --target linux-arm64 --base build
   ```

The `deps` command copies the platform-specific `xray` binary from `distro/<os>/bundle/<arch>/`. Drop the upstream `xray-core` release there before invoking the build so archives stay self-contained. When running from source without the release bundles, copy the appropriate `xray` binary into your installation directory (Windows expects `<install>/bin/xray.exe`, Linux typically uses `/usr/bin/xray` or the OpenWrt package).

### Windows MSI installer

The WiX projects under `installer/wix` bundle `xp2p.exe`, `xray.exe`, and optional bootstrap actions into `xp2p-<version>-windows-amd64.msi`. The quickest way to build and smoke-test the installer is:

```powershell
# From the repo root on Windows
.\scripts\build\build_and_install_msi.ps1 -Version 0.1.5
```

For manual control, compile via `candle.exe`/`light.exe` while pointing `-dXp2pBinary` and `-dXrayBinary` at the artifacts under `build/windows-<arch>/`. Installs accept standard `msiexec` properties such as `INSTALLFOLDER`, `MSIINSTALLPERUSER`, `XP2P_CLIENT_ARGS`, and `XP2P_SERVER_ARGS`.

### Other packages

- Debian packaging, OpenWrt IPKs, and SDK bootstrapping scripts are documented in [`scripts/build/README.md`](scripts/build/README.md).
- The Debian builder VM lives under `infra/vagrant/debian12`, while Windows smoke-test boxes sit in `infra/vagrant/windows10`. Use these environments when you need hermetic packaging workflows.

## Configuration

`xp2p` reads configuration in the following order: built-in defaults > optional file > environment variables > CLI overrides. By default it scans for `xp2p.yaml|yml|toml` in the current directory, or you can pass `--config path/to/file`. Settings map 1:1 to environment variables via the `XP2P_` prefix (`XP2P_SERVER_INSTALL_DIR`, `XP2P_CLIENT_SERVER_ADDRESS`, etc.). See `config_templates/xp2p.example.yaml` for a starting point:

```yaml
logging:
  level: info
  format: text

server:
  port: 62022
  install_dir: C:\xp2p
  config_dir: config-server
  host: edge.example.com

client:
  install_dir: C:\xp2p
  config_dir: config-client
  server_address: edge.example.com
  server_port: 8443
  allow_insecure: true
```

Every command shares global flags such as `--config`, `--log-level`, `--log-json`, `--diag-service-port`, and `--diag-service-mode`. Run `xp2p completion <shell>` to install shell completions or `xp2p docs --dir ./docs/cli` to generate a Markdown command reference straight from the Cobra tree.

## Typical workflows

### Server lifecycle

Server commands manage xray inbound listeners, TLS assets, and user state. A common flow looks like:

```powershell
xp2p server install --path C:\xp2p --host edge.example.com --port 62022 `
  --cert C:\certs\fullchain.pem --key C:\certs\privkey.pem --force
xp2p server run --auto-install --xray-log-file C:\xp2p\logs\xray.err

# Manage users and reverse bridges
xp2p server user add --id branch@example.com --password S3cret --host edge.example.com
xp2p server user list
xp2p server user remove --id branch@example.com

# Networking helpers
xp2p server redirect add --cidr 10.20.0.0/16 --tag trojan-inbound
xp2p server forward add --target 192.0.2.10:22 --proto tcp --listen 127.0.0.1 --listen-port 60022
xp2p server reverse list

# TLS upkeep
xp2p server cert set --cert C:\certs\fullchain.pem --key C:\certs\privkey.pem --host edge.example.com --force
```

`xp2p server state` prints the currently installed assets, while `xp2p server remove --keep-files` verifies presence without deleting anything. All server commands honor `--path`/`--config-dir` overrides so you can stage multiple instances side by side.

### Client lifecycle

Client commands configure Windows workstations, Linux servers, or OpenWrt routers. Release archives already place `xray` next to `xp2p`; if you build from source make sure the platform-appropriate binary exists before running `xp2p client install`.

```bash
# Install from trojan:// link (auto-populates user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com" --force

# Or supply fields manually
xp2p client install \
  --server-address edge.example.com \
  --server-port 62022 \
  --user office@example.com \
  --password PASSWORD \
  --server-name edge.example.com \
  --allow-insecure=false

xp2p client list
xp2p client run --auto-install --xray-log-file /var/log/xp2p/xray.log

# LAN policy helpers
xp2p client redirect add --cidr 192.168.10.0/24 --host edge.example.com
xp2p client redirect add --domain "*.corp.example" --tag trojan-inbound
xp2p client redirect list
xp2p client redirect remove --cidr 192.168.10.0/24

# Inspect reverse bridges and forwards mirrored from the server
xp2p client reverse list
xp2p client forward list
```

`xp2p client remove --all --keep-files` leaves binaries intact but clears configuration, which is useful when repackaging deployments. SOCKS proxy autodetection feeds diagnostics: `xp2p ping example.com --socks` will read the client/server configuration and probe connectivity through the tunnel.

### Remote deploy handshake

`xp2p client deploy` bootstraps a remote host over SSH/RDP-less channels. It emits an encrypted `xp2p+deploy://` link, waits for the server-side listener, pushes state, and then installs the local client using the generated `trojan://` link:

```bash
xp2p client deploy --remote-host branch-gw.example.com --user branch@example.com --trojan-port 62022
```

On the server, run:

```bash
xp2p server deploy --link "xp2p+deploy://ENCODED_PAYLOAD" --listen :62025 --once
```

The deploy listener pulls the manifest (install directory, Trojan port, optional user/password), installs or updates the remote server, and returns a signed client link. Handshakes default to a 10-minute TTL and retry automatically until the server comes online.

## Building & packaging details

- `go/tools/targets matrix` prints the GOOS/GOARCH matrix consumed by CI. Pair it with `targets build` and `targets deps` to cross-compile and gather dependencies locally.
- Release archives are produced by zipping the contents of `build/<target>/`. Use standard tooling (`tar`, `zip`, or the GitHub release workflow) to publish the files named by `targets assets`.
- Windows MSI installers are maintained under `installer/wix`, while helper scripts for MSI, Debian, and OpenWrt IPK generation sit inside `scripts/build/`. Those scripts document the exact prerequisites (WiX, dpkg-dev, OpenWrt SDK) and clean staging directories between runs.
- The Debian builder Vagrant box under `infra/vagrant/debian12` mounts the repository at `/srv/xray-p2p` and ships everything needed for `scripts/build/build_deb_xp2p.sh`.
- OpenWrt packages rely on the SDK assets in `openwrt/` plus `scripts/build/ensure_openwrt_sdk.sh`. Generated packages end up in `build/ipk/` along with repository indexes (`generate_openwrt_indexes.sh`).

## Project layout and further docs

- `go/cmd/xp2p` and `go/internal/...` contain the CLI, installers, deploy logic, and state helpers.
- `config_templates/`, `distro/`, `installer/`, `openwrt/`, and `infra/` provide reference configs, bundled binaries, packaging manifests, and reproducible environments.
- Development, testing, and release guidance lives in [`CONTRIBUTING.md`](CONTRIBUTING.md), [`tests/README.md`](tests/README.md), and [`tests/TESTING_GUIDELINES.md`](tests/TESTING_GUIDELINES.md). Follow those docs for smoke tests, regression suites, and CI conventions.
