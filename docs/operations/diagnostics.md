# Diagnostics & NAT helpers

## Common

- Heartbeat/state: `xp2p client state` and `xp2p server state`.
- Diagnostics responder: `xp2p diag` starts a foreground listener for `xp2p ping`.
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove`.
- DNS/DHCP: `xp2p {client,server} dns-forward add|remove|list`.
- NAT snippets: `xp2p nat-redirect add --cidr 192.168.10.0/24` generates transparent intercept snippets.

## Advanced / troubleshooting

- Watch mode: add `--watch` to `xp2p client|server state` to stream tables with TTL filtering.
- Custom diagnostics port/proto: `xp2p diag --listen 0.0.0.0:62025 --proto udp`.
- Tunnel cascade overrides: `xp2p ping <host> -T <target>`; use `-e <tag>` or `-i <index>` (with `-T`) when multiple endpoints share the same host.
