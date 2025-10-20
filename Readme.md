# Quick Install

## Requirements

- SSH port opened on the target server (default `22` or your custom port).
- Internet access for both the server and the OpenWrt client routers.
- At least 30 MB of free storage on the OpenWrt device for xray binaries/configs.

## Step 1. Prepare the server

Run the server bootstrap script on your server (Debian/Ubuntu/CentOS or similar):

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh | sh -s -- install
```

The script installs xray-core, writes the Trojan inbound configs, and enables the xray-p2p service.

## Step 2. Issue a client credential

Still on the server, generate a Trojan user URL and keep it handy:

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- issue
```

You can re-run the command later with `list` or `remove` to manage clients.

## Step 3. Install the client

On the OpenWrt router, run the client installer and paste the URL from the previous step when prompted:

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh | sh -s -- install
```

This installs xray-core, applies the templates, fetches the server-side state, and restarts the xray-p2p service.

---

## One-command bootstrap (optional)

If you prefer a single guided flow that provisions both sides in one run, execute the helper from the client router:

```bash
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh | sh
```

The script walks through address prompts, connects over SSH to the server, installs everything, and wires up redirects/reverse proxies automatically.

# XRAY-p2p Trojan Tunnel

**At a glance:** This repository delivers a minimal Trojan tunnel based on **xray-core** for OpenWrt. It ships with configuration templates and shell scripts that speed up both server and client provisioning.

---

## What the project provides

- Automates XRAY server and client installation on OpenWrt.
- Generates or installs TLS certificates for inbound listeners.
- Manages Trojan accounts (email/password pairs) and tracks their usage server-side.
- Validates generated configs and restarts the xray-p2p service as needed.

---

## Requirements

- `xray-core` (server and client binaries)
- `jq` for JSON processing inside scripts
- `openssl` or `acme.sh` for certificate handling

---

## Manual server commands

Use these snippets when you need to repeat specific tasks or customize parts of the deployment without re-running the full install scripts.

``` bash
# install dependencies
opkg update && opkg install jq openssl-util
# install server
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh | sh -s -- install
# add user
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh | sh -s -- issue
```
Save user connection URL and paste it when install client

TIPS: if curl not working use `wget -qO- https://raw.githubusercontent.com/USER/REPO/BRANCH/script.sh | sh`
---

## Client quick start

Useful when you want to add redirects or DNS helpers after the main client install.
``` bash
# install dependencies
opkg update && opkg install jq
# on the client router: install, then paste the URL when prompted
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh | sh -s -- install
# setup redirect to XRAY local dokodemo port for a subnet (rerun for more)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/redirect.sh | sh -s -- add
# forward a wildcard domain to a specific upstream DNS server (ports auto-increment from 53331)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/dns_forward.sh | sh -s -- add
```
You can use arguments - `curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/redirect.sh | sh -s -- add $YOUR_CIDR_SUBNET`

To remove a redirect later, run `scripts/redirect.sh remove SUBNET` on the client
or `scripts/redirect.sh remove --all` to drop every subnet at once.
Use `scripts/dns_forward.sh list` to inspect metadata, `scripts/dns_forward.sh add` to create entries, and `scripts/dns_forward.sh remove DOMAIN_MASK` to delete them.

The client installer parses the connection string, writes the templates from `config_templates/client` into `/etc/xray-p2p`, marks the client entry as used on the server, and restarts the xray-p2p service to apply the configuration.

---

## State exchange between server and client

- The server keeps `clients.json` with records such as `{ "id": "uuid", "password": "…", "status": "active" }`.
- A client requests the next unused record, marks it as in use, and writes its details to the outbound config.
- Automations can pull data via `ssh`/`scp` (for example: `ssh root@server 'scripts/lib/user_list.sh'`).

---

## Checks and troubleshooting

- Validate a config: `xray -test -confdir /etc/xray-p2p -format json`.
- Inspect logs: `logread -e xray`.
- From the client, verify egress: `curl --socks5 127.0.0.1:1080 https://ifconfig.me` (replace port if you customized it).

---

## Security notes

- Expose the server only over hardened SSH (keys + firewall rules). Keep the Trojan port open on the WAN side.
- Restrict access to `clients.json` (`chmod 600`) and rotate credentials if you suspect leaks.
- Prefer ACME-issued certificates over self-signed ones whenever possible.

---

## Administration helpers

- `scripts/ssl_cert_create.sh` — minimal helper to issue a self-signed certificate with OpenSSL.
- `scripts/lib/ip_show.sh` — queries multiple sources to determine the server’s public IPv4 address.
- `scripts/client.sh` — manages XRAY client install/remove lifecycle on OpenWrt routers.
- `scripts/lib/user_list.sh` — compares `clients.json` with Trojan inbounds and prints active accounts.
- `scripts/server_user.sh` — lists, issues, or removes clients (`list`, `issue`, `remove` commands) and keeps configs in sync.
- `scripts/redirect.sh` — manages nftables redirects (`list`, `add SUBNET [PORT]`, `remove SUBNET|--all`).
- `scripts/dns_forward.sh` — manages per-domain dokodemo-door DNS inbounds: `add`, `list`, and `remove` while syncing dnsmasq and xray-p2p state.
---

## Ideas for improvement
- Provide an authenticated HTTP API for issuing and revoking clients remotely.
- Add UCI/Netifd glue for first-class OpenWrt service management.
---
