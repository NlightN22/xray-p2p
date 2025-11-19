# XRAY-p2p Trojan Tunnel

XRAY-p2p delivers a minimal Trojan tunnel based on **xray-core** for OpenWrt. The repository ships with configuration templates and shell scripts that automate both server and client provisioning.

## Overview

- Fast bootstrap flows: either a guided one-command setup or explicit server/client steps.
- Manages Trojan user credentials and keeps server and client state in sync.
- Generates TLS certificates or consumes existing ones for inbound listeners.
- Provides helper scripts for redirects, DNS, and troubleshooting on OpenWrt.

---

## Connection topology

```text
      Server LAN                     Internet                    Client LAN
   +---------------+          +-----------------+         +--------------------+
   | Server router |==========| Trojan tunnel   |=========| OpenWrt router     |
   | (xray inbound)|   TLS    | (TLS over TCP)  |   TLS   | (xray client)      |
   +-------+-------+          +-----------------+         +-----+--------------+
           |                                                   |
   +-------+--------+                                   +------+-------+
   | Internal hosts |                                   | LAN devices |
   | (servers etc.) |                                   | PCs / TVs   |
   +----------------+                                   +--------------+
```

- The server hosts the public Trojan inbound and manages user credentials.
- The OpenWrt router establishes the outbound tunnel and publishes local socks/redirect listeners.
- LAN devices stay untouched; traffic is steered through the router via redirects or per-subnet policies.

---

## Quick Start

### Prerequisites

- SSH access to the target server with the port open (default `22`, or your custom port).
- Internet connectivity for the server and every OpenWrt client router.
- At least 30 MB of free storage on the OpenWrt router for xray binaries and configs.
- `curl` available on both devices (fall back to `wget -qO- ... | sh` if required).

### Preferred: Guided bootstrap from the client

Run on the OpenWrt router and follow the prompts. The wizard provisions both sides in one run.

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh | sh
```

The helper walks through address prompts, connects over SSH to the server, installs xray-core, issues a Trojan credential, applies client templates, and wires up redirects and reverse proxies automatically.

### Manual setup (alternative path)

Use this route when you need to run each stage independently or customize pieces along the way.

1. **Prepare the server** on Debian/Ubuntu/CentOS-like hosts:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh | sh -s -- install
   ```
   Installs xray-core, writes Trojan inbound configs, and enables the xray-p2p service.
2. **Issue a client credential** and save the resulting URL:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- issue
   ```
   Use `list` or `remove` with the same script to manage users later on.
3. **Install the client** on OpenWrt:
   ```bash
   opkg update && opkg install jq
   curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh | sh -s -- install
   ```
   Paste the saved URL when prompted. The installer pulls state from the server, writes templates from `config_templates/client`, and restarts xray-p2p.

### Client deployment packages

The Go CLI can build standalone deployment archives so you can inspect or upload them manually:

```bash
xp2p client deploy --remote-host 10.0.10.10 --package-only
```

The command produces a versioned directory with placeholder install scripts and a generated configuration tied to the requested host.

On the remote host, reuse the manifest baked into the package:

```powershell
xp2p server install --deploy-file .\config\deployment.json
```

The CLI pulls the target host/version from the manifest and keeps the installation in sync with the generated package.

## CLI reference

`xp2p` now uses Cobra, so every command ships with contextual `--help` output and sensible flag defaults. The most common flows are:

```bash
# Install or update the server
xp2p server install --path C:\xp2p --host edge.example.com --force

# Run the server in the foreground and auto-install if assets are missing
xp2p server run --auto-install --xray-log-file C:\xp2p\logs\xray.err

# Manage Trojan users
xp2p server user add --id alpha --password secret --host edge.example.com
xp2p server user list --host edge.example.com

# Configure the Windows client
xp2p client install --link trojan://SECRET@edge.example.com:62022
xp2p client run --auto-install
xp2p client redirect add --cidr 10.10.0.0/16 --host edge.example.com
xp2p client redirect add --domain api.example.com --tag proxy-edge-example
xp2p client redirect list
xp2p client redirect remove --domain api.example.com --host edge.example.com

# Forward local services through dokodemo-door listeners (works for both roles)
xp2p client forward add --target 192.0.2.50:22 --listen 127.0.0.1 --proto tcp
xp2p server forward list
xp2p client forward remove --listen-port 53331
xp2p client list
xp2p client remove edge.example.com
xp2p client remove --all --ignore-missing
```

Both the server and client automatically wire up reverse tunnels keyed by sanitized user+host identifiers. Running `xp2p server user add --id alpha@example.com --host edge.example.com ...` now provisions the `<alpha-example-comedge-example-com>.rev` portal/tag, and every `xp2p client install` using that user/server pair creates the mirrored reverse bridge plus routing rules without any manual JSON edits. Use `xp2p server reverse` or `xp2p client reverse` to audit the portals, bridges, endpoint bindings, and routing rules that keep those tunnels alive.


Additional helpers:

- `xp2p client redirect add --cidr <cidr> --host <host>` or `--domain <name>` configures per-destination routing that also shows up in `xp2p client redirect list`. When neither `--tag` nor `--host` is supplied the CLI prints every known client endpoint (tag + host) and prompts you to pick one, keeping Enter as a quick cancel. `xp2p client redirect remove` and the matching `xp2p server redirect add/remove` commands reuse the same interactive picker for reverse portals.
- `xp2p client forward add --target <ip:port>` (or `xp2p server forward add`) provisions a dokodemo-door inbound, auto-picks a listen port from 53331 when `--listen-port` is omitted, and persists the full rule in `install-state-*.json`. The `--proto` flag accepts `tcp`, `udp`, or `both`.
- `xp2p client forward remove --listen-port <port>` / `xp2p server forward remove` removes the matching rule (you can also use `--tag` or `--remark`) and rewrites `inbounds.json`.
- `xp2p client forward list` and `xp2p server forward list` print every forward with listen address/port, protocol set, target, and remark. When the target IP does not fall inside any redirect range the CLI emits a warning so you can add the missing redirect.
- `xp2p server reverse [list]` and `xp2p client reverse [list]` read the relevant `install-state-*.json` and `config-*/routing.json` files, showing the domain/tag, host, user, outbound/endpoint tag, and whether the portal/bridge plus routing rules are present. Running the command without an explicit `list` subcommand defaults to the table view.
- `xp2p ping <host>` runs the diagnostics pinger. For example, `xp2p ping 10.62.10.12 --socks` forces SOCKS5 routing; omit the value to auto-detect the listener from local xp2p configs (client first, then server). When neither config exposes a SOCKS inbound the CLI reports `SOCKS proxy not configured; specify --socks host:port or install xp2p client/server`.
- `xp2p completion [bash|zsh|fish|powershell]` emits shell completion scripts.
- `xp2p docs --dir ./docs/cli` writes Markdown reference files for every command/subcommand via `cobra/doc`.
- `xp2p client list` and `xp2p client remove` help you inspect or prune tunnels without touching the files manually.

### Heartbeat monitoring

- `xp2p server state` reads `state-heartbeat.json` under the server install directory and prints tunnel tag, host, status (alive/dead), RTT metrics, last update, and the reported client IP. Use `--watch` to refresh every few seconds, `--ttl` to tweak the alive threshold (defaults to 10s), and `--interval` to control refresh cadence.
- `xp2p client state` shows the local heartbeat cache so you can confirm the server was reachable recently; the watch/ttl flags mirror the server command.
### Client inventory and cleanup

`xp2p client list` reads `install-state-client.json` and prints every configured tunnel with hostname, outbound tag, remote address, user, server name, and TLS policy. When no endpoints exist it prints `No client endpoints configured.` which keeps automation simple.

`xp2p client remove` requires either a positional `<hostname|tag>` or the `--all` flag:

- `xp2p client remove example.com` removes a single endpoint, deletes redirect rules bound to that outbound tag, and rewrites `outbounds.json`/`routing.json`. If it was the last endpoint, the command transparently falls back to `xp2p client remove --all`.
- `xp2p client remove --all [--keep-files] [--ignore-missing]` keeps the previous behavior and wipes the full config/state tree (with optional flags to keep files or skip missing installs).

Running `xp2p client remove` without a target or `--all` now errors out with a clear hint, so accidental full wipes are avoided.
Both `xp2p client remove` and `xp2p server remove` prompt for confirmation unless `--quiet` is supplied, ensuring unattended scripts can opt out of interactive prompts.

## Installation layout

xp2p keeps configuration and logs in predictable locations and supports running the client and server roles side-by-side.

**Windows (MSI/portable)**

```text
C:\Program Files\xp2p\
  xp2p.exe
  bin\xray.exe
  config-client\
  config-server\
  logs\
  install-state-client.json
  install-state-server.json
```

**Linux / OpenWrt**

```text
/usr/sbin/xp2p                # binary provided by the package manager
/etc/xp2p/
  config-client/
  config-server/
  install-state-client.json
  install-state-server.json
/var/log/xp2p/
  client.log / server.log …
```

- The marker files (`install-state-*.json`) record which roles are active; reinstalling one role no longer overwrites the other.
- Windows bundles `xray.exe` in `bin/`. Linux/OpenWrt expect `xray` to be installed separately (e.g. via opkg) and available in `PATH` or via `XP2P_XRAY_BIN`.
- `--config-dir` continues to work on every platform: relative values are resolved against `C:\Program Files\xp2p` on Windows and `/etc/xp2p` on Linux.
- `--xray-log-file` is still accepted; on Linux relative paths are stored under `/var/log/xp2p`.

When xp2p detects a self-contained layout (for example, a portable Windows unzip), it transparently uses that directory. Otherwise it falls back to the platform defaults described above.

Packaging tools or manual installs must place the `xray` binary under `bin/` ahead of time; `xp2p client install` and `xp2p server install` no longer write or update `xray.exe` themselves and will error out if the binary is missing.

To completely remove the OpenWrt package (together with dependencies), run:

```bash
opkg remove --autoremove xp2p
```

---

## Follow-up tasks on the client

```bash
# Add a subnet redirect to the local dokodemo port (rerun for additional subnets)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/redirect.sh | sh -s -- add

# Forward a wildcard domain to a dedicated upstream DNS resolver (ports auto-increment from 53331)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/dns_forward.sh | sh -s -- add
```

Supply arguments directly if you already know the CIDR or domain mask:

```bash
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/redirect.sh | sh -s -- add 192.168.10.0/24
```

To remove redirects later, run `scripts/redirect.sh remove SUBNET` or `scripts/redirect.sh remove --all`. For DNS forwards use `scripts/dns_forward.sh list`, `add`, or `remove DOMAIN_MASK`.

---

## DNS forwarding

Use DNS forwarding when specific domains must resolve through the tunneled server (for example, to bypass ISP DNS filtering or ensure split-tunnel services use the remote DNS).

```bash
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/dns_forward.sh | sh -s -- add *.example.com 1.1.1.1
```

The script reserves a dokodemo-door inbound, updates dnsmasq to hand out port-specific servers, and restarts xray-p2p/dnsmasq. Each domain mask gets its own listener so you can mix upstream resolvers domain-by-domain.

- `list` shows every forwarded domain and its assigned port.
- `remove DOMAIN_MASK` deletes the listener and dnsmasq entries.

---

## TLS certificates

For production use you should install a trusted certificate instead of relying on the self-signed fallback. A simple path is to issue via `acme.sh` on the server:

```bash
curl -fsSL https://raw.githubusercontent.com/acmesh-official/acme.sh/refs/heads/master/acme.sh | sh
~/.acme.sh/acme.sh --issue --standalone -d vpn.example.com
~/.acme.sh/acme.sh --install-cert -d vpn.example.com \
  --cert-file      /etc/xray-p2p/cert.pem \
  --key-file       /etc/xray-p2p/key.pem \
  --fullchain-file /etc/xray-p2p/fullchain.pem
```

Once the certificate and key are in place, wire them into the XRAY config using the helper:

```bash
scripts/lib/server_install_cert_apply.sh \
  --cert /etc/xray-p2p/cert.pem \
  --key  /etc/xray-p2p/key.pem \
  --inbounds /etc/xray-p2p/inbounds.json
```

The script updates the Trojan inbound with the supplied paths without touching the files themselves, so rerun it after each renewal. If you need to fall back to a local self-signed pair, invoke `scripts/lib/server_install_cert_selfsigned.sh`.

During fresh installs the same paths can be supplied in one go:

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh \
  | sh -s -- install --cert /etc/xray-p2p/cert.pem --key /etc/xray-p2p/key.pem
```

---

## Development prerequisites

- **Go toolchain**: Install Go 1.23.x (tested with 1.23.3) so local builds match
  the OpenWrt SDK toolchain. On Windows either install the official
  `go1.23.3.windows-amd64.msi` or set `GOTOOLCHAIN=go1.23.3` in your shell/VS Code
  settings. Newer Go releases try to bump `go.mod` to 1.24+, which the SDK rejects.
- **Vagrant**: Use the Debian 12 box under `infra/vagrant/debian12` for a reproducible
  OpenWrt SDK environment (runs on VirtualBox with 4 GB RAM by default).

## Debian package sandbox

Use `infra/vagrant/debian12/deb-build` to spin up a Debian 12 builder that already
contains `build-essential`, `debhelper`, `lintian`, `fpm`, and the Go toolchain
required for xp2p.

1. `cd infra/vagrant/debian12/deb-build && vagrant up` – provisions the VM and syncs
   the repository into `/srv/xray-p2p` (and `/home/vagrant/xray-p2p` inside the VM).
2. Run the packaging script:  
   `vagrant ssh -c '/srv/xray-p2p/infra/vagrant/debian12/deb-build/build-deb.sh'`.
   The script builds xp2p, infers the version via `xp2p --version`, and calls FPM.
   Pass `XP2P_DEB_DEPENDS="..."` to declare optional package deps (empty by default).
3. Collect the artefact from the shared folder:
   `build/deb/artifacts/xp2p_<version>_amd64.deb` (visible both on the host and in
   the VM). Use `lintian build/deb/artifacts/*.deb` inside the VM for quick checks.

Re-run `build-deb.sh` any time you need a fresh package; it cleans the staging
directory before building so repeated runs stay deterministic.

Need the upstream `xray` binary on a clean Debian install? Run the helper wrapper
around the official installer:

```bash
sudo scripts/install_xray_core.sh --install
```

The wrapper simply downloads `https://github.com/XTLS/Xray-install` and hands all
arguments through to the upstream script, so check that project for supported
flags/versions.

When you want to ship a Linux `xray` binary together with xp2p (for example inside
the `.deb`), place it under `distro/linux/bundle/<arch>/xray` using the same
layout as the Windows bundle (`distro/windows/bundle/x86_64/xray.exe`). Leave the
OpenWrt builds external so their packages stay lean.

---

## Manual operations

### Server maintenance

- Install or reinstall server components:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh | sh -s -- install
  ```
- Manage Trojan users without reinstalling:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- list
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- issue
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- remove you@example.com
  ```

### Client maintenance

- Re-run the installer to refresh configs:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh | sh -s -- install
  ```
- Remove the client stack if needed:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh | sh -s -- remove
  ```

---

## Contributing

Developer-focused docs (tests, CI, release flow) live in [`CONTRIBUTING.md`](CONTRIBUTING.md). The Windows smoke-test environment is still covered in [`tests/README.md`](tests/README.md).

## How server and client stay in sync

- The server stores credentials in `clients.json` as records like `{ "id": "uuid", "password": "...", "status": "active" }`.
- When a client installs, it requests the next unused record, marks it as consumed, and writes the details to its outbound config.
- Automations can pull data via SSH, for example: `ssh root@server 'scripts/lib/user_list.sh'`.

---

## Troubleshooting

- Validate the combined config: `xray -test -confdir /etc/xray-p2p -format json`.
- Review logs on OpenWrt: `logread -e xray`.
- Confirm outbound connectivity: `curl --socks5 127.0.0.1:1080 https://ifconfig.me` (adjust port if customized).

---

## Windows MSI installer

A signed Windows release now includes `xp2p-<version>-windows-amd64.msi` (and its `latest` companion). The installer copies `xp2p.exe` into `C:\Program Files\xp2p` for elevated sessions and automatically falls back to `%LOCALAPPDATA%\xp2p` when the user does not hold administrative privileges. Repairs (`msiexec /fa`) restore the binary if it becomes corrupted, and uninstalls purge all files left in the installation directory.

### Optional CLI bootstrap

- Pass `XP2P_CLIENT_ARGS="--link trojan://... --force"` to run `xp2p client install ...` right after the binary lands on disk.
- Pass `XP2P_SERVER_ARGS="--deploy-file C:\xp2p\config\deployment.json"` to execute `xp2p server install ...`.
- Both custom actions run only during a fresh install, so repairs and uninstalls are unaffected.

### Silent, custom, and per-user installs

- Override the target directory explicitly: `msiexec /i xp2p-<version>-windows-amd64.msi INSTALLFOLDER="D:\Network\xp2p"`.
- Force a per-user install even when you have elevation: `msiexec /i xp2p-<version>-windows-amd64.msi MSIINSTALLPERUSER=1`.
- Fully silent automation examples:
  - Install: `msiexec /i xp2p-<version>-windows-amd64.msi /qn XP2P_CLIENT_ARGS="--link trojan://secret@example.com:62022"`
  - Repair: `msiexec /fa xp2p-<version>-windows-amd64.msi /qn`
  - Uninstall: `msiexec /x xp2p-<version>-windows-amd64.msi /qn`

### Building the MSI locally

1. Install the WiX Toolset (`choco install wixtoolset --no-progress -y`) and make sure `candle.exe`/`light.exe` sit on `PATH`.
2. Build the Windows binary with the embedded version:  
   `go run ./go/tools/targets build --target windows-amd64 --base build --binary xp2p --pkg ./go/cmd/xp2p --ldflags "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$(go run ./go/cmd/xp2p --version)"`
3. Copy the matching `xray.exe` into `distro/windows/bundle/x86_64/xray.exe` (the MSI build script copies it into the staging directory automatically).
4. Compile the WiX project from `installer/wix/xp2p.wxs`:
   ```powershell
   candle -dProductVersion=<version> `
          -dXp2pBinary=build/windows-amd64/xp2p.exe `
          -dXrayBinary=distro/windows/bundle/x86_64/xray.exe `
          installer/wix/xp2p.wxs
   light -out dist/xp2p-<version>-windows-amd64.msi installer/wix/xp2p.wixobj
   ```
   The helper `scripts/build/build_and_install_msi.ps1` automates the amd64 build/install flow if you prefer a single command.
   For 32-bit systems use `installer/wix/xp2p-x86.wxs`, `distro/windows/bundle/x86/xray.exe`, and the helper script `scripts/build/build_and_install_msi_x86.ps1`.

## Security notes

- Expose the server only over hardened SSH (keys plus firewall rules). Keep the Trojan port open on the WAN side.
- Restrict access to `clients.json` (`chmod 600`) and rotate credentials if compromise is suspected.
- Prefer ACME-issued certificates over self-signed ones whenever possible.

---

## Helper scripts

- `scripts/ssl_cert_create.sh` - minimal helper to issue a self-signed certificate with OpenSSL.
- `scripts/lib/ip_show.sh` - determines the server public IPv4 address via multiple sources.
- `scripts/client.sh` - manages XRAY client install/remove lifecycle on OpenWrt routers.
- `scripts/lib/user_list.sh` - compares `clients.json` with Trojan inbounds and prints active accounts.
- `scripts/server_user.sh` - lists, issues, or removes clients and keeps configs in sync.
- `scripts/lib/server_install_cert_apply.sh` - applies existing certificate/key paths to trojan inbounds.
- `scripts/lib/server_install_cert_selfsigned.sh` - generates or refreshes a self-signed certificate.
- `scripts/redirect.sh` - manages nftables redirects (`list`, `add SUBNET [PORT]`, `remove SUBNET|--all`).
- `scripts/dns_forward.sh` - manages per-domain dokodemo-door DNS inbounds (`add`, `list`, `remove`).

---

## Ideas for improvement

- Provide an authenticated HTTP API for issuing and revoking clients remotely.
- Integrate `acme.sh` for automated certificate issuance and renewal.
- Add UCI/Netifd glue for first-class OpenWrt service management.
