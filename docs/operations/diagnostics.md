# Diagnostics & NAT helpers

- Heartbeat/state: `xp2p client state --watch` and `xp2p server state --watch` stream heartbeat tables from `state-heartbeat.json` with TTL filtering.
- Diagnostics responder: `xp2p diag` starts a foreground listener for `xp2p ping`; override with `xp2p diag --listen 0.0.0.0:62025 --proto udp` if you need a custom port/protocol.
- Tunnel cascade: `xp2p ping 10.62.10.12 -T` auto-detects SOCKS from client config, then server, then errors if absent; override with `-T 127.0.0.1:1080`. Use `-e <tag>` or `-i <index>` (with `-T`) to select a client endpoint when multiple endpoints share the same host. For reverse channels, pass the reverse user/tag as the host (for example `xp2p ping deploy-123@local -T` or `xp2p ping proxy-10-62-10-11 -T`).
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove` manage explicit forwards alongside managed reverse portals.
- DNS/DHCP: `xp2p {client,server} dns-forward add|remove|list` manage per-domain entries in dnsmasq (`dhcp.@dnsmasq[0].server`) on OpenWrt and keep state in sync.
- NAT snippets: `xp2p nat-redirect add --cidr 192.168.10.0/24` sets up transparent intercept snippets and validates nft/iptables chains; rerun to regenerate directories if missing.
