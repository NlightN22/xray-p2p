# CLI workflows

This page is a command-oriented cheat sheet. For conceptual details, see [Deploy flow](../flows/deploy-flow.md) and [Apply flow](../flows/apply-flow.md).

## Server lifecycle

Server commands manage xray inbound listeners, TLS assets, and user state. A common flow looks like:

```bash
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
xp2p server dns-forward add --domain corp.example --target 10.10.10.53:53
xp2p server dns-forward list

# TLS upkeep
xp2p server cert set --cert /path/to/fullchain.pem --key /path/to/privkey.pem
xp2p server cert state
```

## Client lifecycle

Client commands configure OpenWrt routers, Linux hosts, or Windows workstations. Release archives already place `xray` next to `xp2p`, so keep both binaries together when copying the installation directory between hosts.

```bash
# Install from trojan:// link (auto-populates user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"

# Or supply fields manually
xp2p client install --host edge.example.com --port 62022 --user office@example.com --password PASSWORD --allow-insecure

# Choose target mode during install
xp2p client install --mode proxy --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"
xp2p client install --mode tun --tun-mode full --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"

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
xp2p client dns-forward add --domain dev.example --target 10.10.10.53:53
xp2p client dns-forward list
xp2p client dns-forward remove --domain dev.example
```

`xp2p client remove --all` removes the client configuration and binaries, which is useful when repackaging deployments.
