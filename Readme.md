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
- Remove: `opkg remove xp2p` (stops services and removes the package). To purge, delete `/etc/xp2p`, `/var/log/xp2p`, and the init scripts.
- Optional manual feed setup: `echo "src-git xp2p https://github.com/NlightN22/xray-p2p.git;main" >> /etc/opkg/customfeeds.conf && opkg update && opkg install xp2p`.
- From a local IPK: `opkg install /tmp/xp2p_<version>_<arch>.ipk`.

### Linux (Debian/Ubuntu packages)

- Grab the `.deb` for your arch from the Release (`xp2p_<version>_amd64.deb`, `xp2p_<version>_arm64.deb`, `xp2p_<version>_armhf.deb`, `xp2p_<version>_386.deb`).
- Install: `sudo dpkg -i xp2p_<version>_<arch>.deb || sudo apt-get -f install`.
- Binaries land under `/usr/bin/xp2p`, bundled `xray` under `/etc/xp2p/bin/xray`, configs under `/etc/xp2p/config-{client,server}`, logs under `/var/log/xp2p`.
- Services: `systemctl enable --now xp2p-client xp2p-server` (wrap `xp2p client|server service run` with default flags).
- Remove: `sudo dpkg -r xp2p`; purge data with `sudo dpkg -P xp2p`.
- Alternative (archives): download `xp2p-<version>-<target>.tar.gz`, unpack, keep `xp2p` next to bundled `xray`, and add the directory to `PATH`.

### Windows

- Download `xp2p-<version>-windows-amd64.msi` (or the `.zip` archive).
- Install MSI with standard commands:
  ```powershell
  msiexec /i xp2p-<version>-windows-amd64.msi
  msiexec /x xp2p-<version>-windows-amd64.msi
  ```
- Services `xp2p-client` and `xp2p-server` wrap `xp2p client|server service run`; manage via `xp2p client service start|stop|status` or the Services snap-in. Logs: `C:\Program Files\xp2p\logs\<role>\`.

Need to build from source or generate packages? Follow [`scripts/build/README.md`](scripts/build/README.md).

## Platform quick start

### OpenWrt

```sh
opkg update && opkg install xp2p
xp2p server install --host edge.example.com --port 62022
xp2p client install --host edge.example.com --port 62022 --user office@example.com --password PASS --allow-insecure
service xp2p-server start
service xp2p-client start
xp2p server state
```

Use `xp2p server dns-forward add --domain corp.example --address 10.10.10.53` and `xp2p client dns-forward list` to program dnsmasq on OpenWrt; the helpers are idempotent and clean up on remove. `xp2p nat-redirect apply` bootstraps `/etc/nftables.d` (or `/etc/xp2p/nftables` fallback) so NAT snippets come up without manual directories.

### Linux

```bash
sudo dpkg -i xp2p_<version>_amd64.deb || sudo apt-get -f install
xp2p server install --host edge.example.com --port 62022
xp2p client install --host edge.example.com --port 62022 --user office@example.com --password PASS --allow-insecure
sudo systemctl enable --now xp2p-server xp2p-client
xp2p server state
```

Add CIDR/domain redirects and forwards after install:
```bash
xp2p client redirect add --cidr 192.168.10.0/24
xp2p client forward add --target 192.0.2.10:22
xp2p client dns-forward add --domain dev.example --address 10.10.10.53
xp2p nat-redirect apply
```

### Windows

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
xp2p server install --host edge.example.com --port 62022
xp2p client install --host edge.example.com --port 62022 --user office@example.com --password PASS --allow-insecure
xp2p client service start
xp2p server service start
xp2p client reverse list
```

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
xp2p server install --host edge.example.com --port 62022
xp2p server run

# Manage users and reverse bridges
xp2p server user add --id branch@example.com --password S3cret
xp2p server user list
xp2p server user remove --id branch@example.com
xp2p server reverse list

# Networking helpers
xp2p server redirect add --cidr 10.20.0.0/16
xp2p server forward add --target 192.0.2.10:22
xp2p server reverse list
xp2p server dns-forward add --domain corp.example --address 10.10.10.53
xp2p server dns-forward list

# TLS upkeep
xp2p server cert set --cert C:\certs\fullchain.pem --key C:\certs\privkey.pem
xp2p server cert state
```

`xp2p server state` prints the currently installed assets, while `xp2p server remove` removes the installation after confirmation.
`xp2p server cert state` reports the active TLS certificate (paths, SAN, validity) and exits with 0 only when a valid certificate is present.

### Client lifecycle

Client commands configure OpenWrt routers, Linux hosts, or Windows workstations. Release archives already place `xray` next to `xp2p`, so keep both binaries together when copying the installation directory between hosts.

```bash
# Install from trojan:// link (auto-populates user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"

# Or supply fields manually
xp2p client install \
  --host edge.example.com \
  --port 62022 \
  --user office@example.com \
  --password PASSWORD \
  --allow-insecure

xp2p client list
xp2p client run

# LAN policy helpers
xp2p client redirect add --cidr 192.168.10.0/24
xp2p client redirect add --domain "*.corp.example"
xp2p client redirect remove --cidr 192.168.10.0/24

# Forwards and reverse tunnels
xp2p client forward add --target 192.0.2.10:22
xp2p client forward list
xp2p client reverse list

# DNS/DHCP helpers
xp2p client dns-forward add --domain dev.example --address 10.10.10.53
xp2p client dns-forward list
xp2p client dns-forward remove --domain dev.example --address 10.10.10.53
```

`xp2p client remove --all` removes the client configuration and binaries, which is useful when repackaging deployments. Tunnel proxy autodetection feeds diagnostics: `xp2p ping example.com --tunnel` reads the client/server configuration and probes connectivity through the tunnel. Add `--endpoint <tag>` to force the client-side outbound (host is ignored when tag is provided) or pass the reverse user/tag on the server (or `--endpoint <user>`) to route via the reverse channel; then `xp2p nat-redirect apply` bootstraps nftables/iptables snippets if you want transparent interception on Linux/OpenWrt.

### Remote deploy handshake

`xp2p client deploy` bootstraps a remote host over SSH/RDP-less channels. It emits a single `trojan://` deploy link (with user/password and extra tokens), waits for the server-side listener, pushes state, and then installs the local client using the generated `trojan://` link:

```bash
xp2p client deploy --host branch-gw.example.com --user branch@example.com
```

On the server, run:

```bash
xp2p server deploy --link "trojan://PASSWORD@branch-gw.example.com:62022?deploy_version=2&exp=1763743202&security=tls&sni=branch-gw.example.com#branch@example.com"
```

The server stops listening after the first deploy request. The client encrypts its deploy manifest with a key derived from the trojan link, so only ciphertext crosses the wire. The deploy listener decrypts the payload, verifies it matches the link you supplied, installs or updates the remote server, and returns a signed client link. Handshakes default to a 10-minute TTL and retry automatically until the server comes online.

## Diagnostics, routing, and NAT helpers

- Heartbeat/state: `xp2p client state --watch` and `xp2p server state --watch` stream heartbeat tables from `state-heartbeat.json` with TTL filtering.
- Tunnel cascade: `xp2p ping 10.62.10.12 --tunnel` auto-detects SOCKS from client config, then server, then errors if absent; override with `--tunnel 127.0.0.1:1080`. Use `--endpoint <tag>` on the client to force a specific outbound regardless of host, or pass the reverse user/tag (or `--endpoint <user>`) on the server to select a reverse channel.
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove` manage explicit forwards alongside managed reverse portals.
- DNS/DHCP: `xp2p {client,server} dns-forward add|remove|list` manage per-domain entries in dnsmasq (`dhcp.@dnsmasq[0].server`) on OpenWrt and keep state in sync.
- NAT snippets: `xp2p nat-redirect apply` sets up transparent intercept snippets and validates nft/iptables chains; rerun to regenerate directories if missing.

## Project layout and further docs

- `go/cmd/xp2p` and `go/internal/...` contain the CLI, installers, deploy logic, and state helpers.
- `config_templates/`, `distro/`, `installer/`, `openwrt/`, and `infra/` provide reference configs, bundled binaries, packaging manifests, and reproducible environments.
- Development, testing, and release guidance lives in [`CONTRIBUTING.md`](CONTRIBUTING.md), [`tests/README.md`](tests/README.md), and [`tests/TESTING_GUIDELINES.md`](tests/TESTING_GUIDELINES.md). Follow those docs for smoke tests, regression suites, and CI conventions.
