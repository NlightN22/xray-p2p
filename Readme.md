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
```

Additional helpers:

- `xp2p ping <host>` runs the diagnostics pinger (`--socks` accepts either a value or falls back to the config default).
- `xp2p completion [bash|zsh|fish|powershell]` emits shell completion scripts.
- `xp2p docs --dir ./docs/cli` writes Markdown reference files for every command/subcommand via `cobra/doc`.

## Installation layout

xp2p now ships as a single self-contained directory. Every installation follows the same structure:

```text
<install-dir>/
  xp2p(.exe)
  install-state.json
  logs/
  bin/
    xray(.exe)
  config-client/
  config-server/
```

- `install-state.json` tracks independent `client` and `server` role markers, so both installations can coexist in the same directory without forcing re-installs.
- The `xp2p` binary always lives at the root next to `install-state.json`, so running `xp2p` from that directory automatically discovers the configs/logs without extra flags.
- `bin/` stores only the xray-core runtime that the CLI manages.
- `config-client/` and `config-server/` contain the rendered JSON configs for each role.
- `logs/` is created during install so that `--xray-log-file logs\<name>.err` always resolves within the tree.

Default install locations:

- **Windows** – `C:\Program Files\xp2p` when the directory is writable; otherwise `%LOCALAPPDATA%\xp2p`.
- **Linux** – `/opt/xp2p` for root sessions, or `$HOME/.local/share/xp2p` for unprivileged users.

When xp2p detects that it is already running from an installation root (for example, a portable unzip), it transparently uses that directory for all commands. MSI/PKG installers only need to copy the tree into place; xp2p will keep configs and logs alongside itself unless explicit `--path`/`--config-dir` flags are provided.

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
3. Compile the WiX project from `installer/wix/xp2p.wxs`:
   ```powershell
   candle -dProductVersion=<version> -dXp2pBinary=build/windows-amd64/xp2p.exe installer/wix/xp2p.wxs
   light -out dist/xp2p-<version>-windows-amd64.msi installer/wix/xp2p.wixobj
   ```

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
