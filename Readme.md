# XRAY-p2p (Go)

XRAY-p2p delivers a cross-platform Trojan tunnel built on top of `xray-core`. The Go-based `xp2p` CLI owns server and client provisioning, state tracking, TLS assets, and helper routes on Windows, Linux, and OpenWrt so you no longer need to depend on the original shell automation.

> Need the legacy shell workflow? The archived text lives in [Readme-old.md](Readme-old.md).

## What xp2p provides

- A single statically linked CLI (`xp2p`) with Cobra-based help, completions, doc generation, and a background diagnostics service.
- Server management covering installation, upgrades, TLS deployment, user provisioning, redirect/forward/reverse bridges, and `xray-core` log collection.
- Client management on Windows, Linux, and OpenWrt including endpoint installs from `trojan://` links, reverse portals, SOCKS autodiscovery, redirects, DNS-aware forwarding, and transparent NAT helpers.
- Remote deployment handshakes (`xp2p client deploy` + `xp2p server deploy`) that ship ready-to-use manifests over an encrypted link before bootstrapping both sides.
- Build tooling that emits per-OS packages together with vendor-supplied `xray` binaries, Windows MSI installers, Debian packages, and OpenWrt IPKs (publishable via `feeds.conf`).

## Getting xp2p

### OpenWrt

- One-line installer (auto-detects release/arch, adds feed/signing key, installs package):
  ```sh
  wget -qO- https://nlightn22.github.io/xray-p2p/install-xp2p.sh | sh
  ```
- Services land under `/etc/init.d/xp2p-client` and `/etc/init.d/xp2p-server` and run `xp2p client|server service run` with default flags.
- Manage lifecycle: `service xp2p-client start|stop|restart|status` or `xp2p client service start|status`; logs live in `/var/log/xp2p/`.
- Remove: `opkg remove xp2p` (stops services); `opkg remove --force-removal-of-dependent-packages xp2p` purges `/etc/xp2p`, `/var/log/xp2p`, and init scripts.
- Optional manual feed setup: `echo "src-git xp2p https://github.com/NlightN22/xray-p2p.git;main" >> /etc/opkg/customfeeds.conf && opkg update && opkg install xp2p`.
- From a local IPK: `opkg install /tmp/xp2p_<version>_<arch>.ipk`.

### Linux archives

- Download `xp2p-<version>-<target>.tar.gz` from Releases (targets: `linux-amd64`, `linux-386`, `linux-arm64`, `linux-armhf`; additional MIPS/RISC-V can be built locally).
- Unpack and keep `xp2p` next to the bundled `xray` binary, then add the directory to `PATH` or point services to it.
- Optional packages (`.deb`) and build scripts are described in [`scripts/build/README.md`](scripts/build/README.md).

### Windows

- Download `xp2p-<version>-windows-amd64.msi` (or the `.zip` archive).
- Install MSI with standard commands:
  ```powershell
  msiexec /i xp2p-<version>-windows-amd64.msi /qn
  msiexec /x xp2p-<version>-windows-amd64.msi
  msiexec /i xp2p-<version>-windows-amd64.msi INSTALLFOLDER="D:\Network\xp2p"
  ```
- Optional MSI properties: `XP2P_CLIENT_ARGS` / `XP2P_SERVER_ARGS` to auto-run installs, `MSIINSTALLPERUSER=1` for per-user mode.
- Services `xp2p-client` and `xp2p-server` wrap `xp2p client|server service run`; manage via `xp2p client service start|stop|status` or the Services snap-in. Logs: `C:\Program Files\xp2p\logs\<role>\`.

Need to build from source or generate packages? Follow [`scripts/build/README.md`](scripts/build/README.md).

## Platform quick start

### OpenWrt

```sh
opkg update && opkg install xp2p
xp2p server install --host edge.example.com --port 62022 --config-dir config-server --force
xp2p client install --host edge.example.com --port 62022 --user office@example.com --password PASS --allow-insecure=true --config-dir config-client --force
service xp2p-server start
service xp2p-client start
xp2p server state --watch --once
```

Use `xp2p server dns-forward add --domain corp.example --address 10.10.10.53` and `xp2p client dns-forward list` to program dnsmasq on OpenWrt; the helpers are idempotent and clean up on remove. `xp2p nat-redirect apply --backend nftables` bootstraps `/etc/nftables.d` (or `/etc/xp2p/nftables` fallback) so NAT snippets come up without manual directories.

### Linux

```bash
tar -xzf xp2p-<version>-linux-amd64.tar.gz -C /usr/local/bin
xp2p server install --path /etc/xp2p --host edge.example.com --port 62022 --force
xp2p client install --path /etc/xp2p --host edge.example.com --port 62022 --user office@example.com --password PASS --sni edge.example.com --allow-insecure=false
xp2p server run --auto-install --xray-log-file /var/log/xp2p/xray-server.log
xp2p client run --auto-install --xray-log-file /var/log/xp2p/xray-client.log
```

Add CIDR/domain redirects and forwards after install:
```bash
xp2p client redirect add --cidr 192.168.10.0/24 --tag proxy-edge
xp2p client forward add --target 192.0.2.10:22 --listen 127.0.0.1 --listen-port 60022 --remark "ssh jump"
xp2p client dns-forward add --domain dev.example --address 10.10.10.53
xp2p nat-redirect apply --backend nftables
```

### Windows

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi /qn
xp2p server install --path "C:\xp2p" --host edge.example.com --port 62022 --force
xp2p client install --path "C:\xp2p" --host edge.example.com --port 62022 --user office@example.com --password PASS --allow-insecure=true
xp2p client service start
xp2p server service start
xp2p client reverse list
```

MSI properties `XP2P_CLIENT_ARGS` / `XP2P_SERVER_ARGS` can auto-run the installs during setup.

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
  diag_port: 62023
  allow_insecure: true
```

Every command shares global flags such as `--config`, `--log-level`, `--log-json`, `--diag-service-port`, and `--diag-service-mode`. Run `xp2p completion <shell>` to install shell completions or `xp2p docs --dir ./docs/cli` to generate a Markdown command reference straight from the Cobra tree.

By default the xp2p server diagnostics responder listens on TCP/UDP port `62022`, while the client-side diagnostics service uses `62023` to avoid conflicts on hosts that run both roles. Override them through the configuration (`server.port` / `client.diag_port`) or environment variables when needed.

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
xp2p server reverse list

# Networking helpers
xp2p server redirect add --cidr 10.20.0.0/16 --tag trojan-inbound
xp2p server forward add --target 192.0.2.10:22 --proto tcp --listen 127.0.0.1 --listen-port 60022
xp2p server reverse list
xp2p server dns-forward add --domain corp.example --address 10.10.10.53
xp2p server dns-forward list

# TLS upkeep
xp2p server cert set --cert C:\certs\fullchain.pem --key C:\certs\privkey.pem --host edge.example.com --force
```

`xp2p server state` prints the currently installed assets, while `xp2p server remove --keep-files` verifies presence without deleting anything. All server commands honor `--path`/`--config-dir` overrides so you can stage multiple instances side by side.

### Client lifecycle

Client commands configure OpenWrt routers, Linux hosts, or Windows workstations. Release archives already place `xray` next to `xp2p`, so keep both binaries together when copying the installation directory between hosts.

```bash
# Install from trojan:// link (auto-populates user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com" --force

# Or supply fields manually
xp2p client install \
  --host edge.example.com \
  --port 62022 \
  --user office@example.com \
  --password PASSWORD \
  --sni edge.example.com \
  --allow-insecure=false

xp2p client list
xp2p client run --auto-install --xray-log-file /var/log/xp2p/xray.log

# LAN policy helpers
xp2p client redirect add --cidr 192.168.10.0/24 --tag proxy-edge
xp2p client redirect add --domain "*.corp.example" --tag proxy-edge
xp2p client redirect remove --cidr 192.168.10.0/24

# Forwards and reverse tunnels
xp2p client forward add --target 192.0.2.10:22 --listen 127.0.0.1 --listen-port 60022 --remark "ssh jump"
xp2p client forward list
xp2p client reverse list

# DNS/DHCP helpers
xp2p client dns-forward add --domain dev.example --address 10.10.10.53
xp2p client dns-forward list
xp2p client dns-forward remove --domain dev.example --address 10.10.10.53
```

`xp2p client remove --all --keep-files` leaves binaries intact but clears configuration, which is useful when repackaging deployments. SOCKS proxy autodetection feeds diagnostics: `xp2p ping example.com --socks` will read the client/server configuration and probe connectivity through the tunnel, and `xp2p nat-redirect apply --backend nftables` bootstraps nftables/iptables snippets if you want transparent interception on Linux/OpenWrt.

### Remote deploy handshake

`xp2p client deploy` bootstraps a remote host over SSH/RDP-less channels. It emits a single `trojan://` deploy link (with user/password and extra tokens), waits for the server-side listener, pushes state, and then installs the local client using the generated `trojan://` link:

```bash
xp2p client deploy --host branch-gw.example.com --user branch@example.com --trojan-port 62022
```

On the server, run:

```bash
xp2p server deploy --link "trojan://PASSWORD@branch-gw.example.com:62022?deploy_version=2&exp=1763743202&security=tls&sni=branch-gw.example.com#branch@example.com" --listen :62025
```

The server stops listening after the first deploy request. The client encrypts its deploy manifest with a key derived from the trojan link, so only ciphertext crosses the wire. The deploy listener decrypts the payload, verifies it matches the link you supplied, installs or updates the remote server, and returns a signed client link. Handshakes default to a 10-minute TTL and retry automatically until the server comes online.

## Diagnostics, routing, and NAT helpers

- Heartbeat/state: `xp2p client state --watch --once` and `xp2p server state --watch` stream heartbeat tables from `state-heartbeat.json` with TTL filtering.
- SOCKS cascade: `xp2p ping 10.62.10.12 --socks` auto-detects SOCKS from client config, then server, then errors if absent; override with `--socks 127.0.0.1:1080`.
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove` manage explicit forwards alongside managed reverse portals.
- DNS/DHCP: `xp2p {client,server} dns-forward add|remove|list` manage per-domain entries in dnsmasq (`dhcp.@dnsmasq[0].server`) on OpenWrt and keep state in sync.
- NAT snippets: `xp2p nat-redirect apply --backend nftables` (or `--backend iptables`) sets up transparent intercept snippets and validates nft/iptables chains; rerun to regenerate directories if missing.

## Project layout and further docs

- `go/cmd/xp2p` and `go/internal/...` contain the CLI, installers, deploy logic, and state helpers.
- `config_templates/`, `distro/`, `installer/`, `openwrt/`, and `infra/` provide reference configs, bundled binaries, packaging manifests, and reproducible environments.
- Development, testing, and release guidance lives in [`CONTRIBUTING.md`](CONTRIBUTING.md), [`tests/README.md`](tests/README.md), and [`tests/TESTING_GUIDELINES.md`](tests/TESTING_GUIDELINES.md). Follow those docs for smoke tests, regression suites, and CI conventions.
