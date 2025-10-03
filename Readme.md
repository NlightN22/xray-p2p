# XRAY-p2p Trojan Tunnel

**At a glance:** This repository delivers a minimal Trojan tunnel based on **xray-core** for OpenWrt. It ships with configuration templates and shell scripts that speed up both server and client provisioning.

---

## What the project provides

- Automates XRAY server and client installation on OpenWrt.
- Generates or installs TLS certificates for inbound listeners.
- Manages Trojan accounts (email/password pairs) and tracks their usage server-side.
- Validates generated configs and restarts the XRAY service as needed.

---

## Requirements

- `xray-core` (server and client binaries)
- `jq` for JSON processing inside scripts
- `openssl` or `acme.sh` for certificate handling

---

## Fast start

``` bash
# install dependencies
opkg update && opkg install jq openssl-util
# install server
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/install_server.sh | sh
# add client
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/issue_client.sh | sh
```
Save client connection URL and paste it when install client

TIPS: if curl not working use `wget -qO- https://raw.githubusercontent.com/USER/REPO/BRANCH/script.sh | sh`
---

## Client quick start
``` bash
# install dependencies
opkg update && opkg install jq openssl-util
# on the client router: install, then paste the URL when prompted
curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/install_client.sh | sh
# setup redirect to XRAY local dokodemo port for a subnet (rerun for more)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xray_redirect.sh | sh
# forward a wildcard domain to a specific upstream DNS server (ports auto-increment from 53331)
curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xray_dns_forward.sh | sh
```
You can use arguments - `curl -s https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xray_redirect.sh | sh -s -- $YOR_CIDR_SUBNET`

To remove a redirect later, run `scripts/xray_redirect_remove.sh` on the client
with either a CIDR (e.g. `10.0.101.0/24`) or `--all` to drop every subnet at once.
To list or remove DNS forwardings, call `scripts/xray_dns_forward_remove.sh` with a mask, `--all`, or `--list` to inspect what's configured.

The client installer parses the connection string, writes the templates from `config_templates/client` into `/etc/xray`, marks the client entry as used on the server, and restarts XRAY to apply the configuration.

---

## State exchange between server and client

- The server keeps `clients.json` with records such as `{ "id": "uuid", "password": "…", "status": "active" }`.
- A client requests the next unused record, marks it as in use, and writes its details to the outbound config.
- Automations can pull data via `ssh`/`scp` (for example: `ssh root@server 'scripts/list_clients.sh'`).

---

## Checks and troubleshooting

- Validate a config: `xray -test -c /etc/xray/config.json`.
- Inspect logs: `/var/log/xray/*` or `logread | grep xray`.
- From the client, verify egress: `curl --socks5 127.0.0.1:1080 https://ifconfig.me` (replace port if you customized it).

---

## Security notes

- Expose the server only over hardened SSH (keys + firewall rules). Keep the Trojan port open on the WAN side.
- Restrict access to `clients.json` (`chmod 600`) and rotate credentials if you suspect leaks.
- Prefer ACME-issued certificates over self-signed ones whenever possible.

---

## Administration helpers

- `scripts/create.ssl.cert.sh` — minimal helper to issue a self-signed certificate with OpenSSL.
- `scripts/pub_ip.sh` — queries multiple sources to determine the server’s public IPv4 address.
- `scripts/install_client.sh` — installs XRAY on an OpenWrt client and applies the provided Trojan URL.
- `scripts/list_clients.sh` — compares `clients.json` with Trojan inbounds and prints active accounts.
- `scripts/remove_client.sh` — revokes a client, updates configs, and restarts XRAY.
- `scripts/xray_redirect.sh` — sets up nftables redirection for a subnet to the local dokodemo-door inbound; repeated runs add more subnets.
- `scripts/xray_dns_forward.sh` — adds per-domain dokodemo-door DNS inbounds, auto-allocates ports from 53331, and syncs dnsmasq forwarding.
- `scripts/xray_dns_forward_remove.sh` — lists (`--list`) or removes domain forwardings (single mask or `--all`) and cleans dnsmasq/xray state.
- `scripts/xray_redirect_remove.sh` — removes redirect entries (`--all` for everything, or pass a CIDR to delete just one).

---

## Ideas for improvement

- Provide an authenticated HTTP API for issuing and revoking clients remotely.
- Integrate `acme.sh` for automated certificate issuance and renewal.
- Add UCI/Netifd glue for first-class OpenWrt service management.

---
