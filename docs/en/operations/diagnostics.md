# Diagnostics & NAT helpers

## Common

- Heartbeat/state: `xp2p client state` and `xp2p server state`.
- Pending view: add `--pending` to show configured tunnels before the service applies changes.
- Diagnostics responder: `xp2p diag` starts a foreground listener for `xp2p ping`.
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove`.
- DNS/DHCP: `xp2p {client,server} dns-forward add|remove|list`.
- NAT snippets: `xp2p nat-redirect add --cidr 192.168.10.0/24` generates transparent intercept snippets.

## Ping checks

`xp2p ping` is a protocol-level connectivity check for the diagnostics responder.
It works when the remote side runs xp2p (client/server service) or when you run `xp2p diag` on that node.

If you do not use this project end-to-end, `xp2p ping` can still work as long as `xp2p diag` is running on the node where Xray is deployed (or on any node you want to probe).

Direct ping (no tunnel, defaults to TCP/62022):

```sh
xp2p ping <host>
```

Reverse/tunnel ping (through the xp2p SOCKS tunnel):

```sh
xp2p ping <host> --tunnel
```

When tunnel mode is used, xp2p may route the probe through an internal marker target. For reverse channels the marker port is different (62023) and is selected automatically.

## Advanced / troubleshooting

- Watch mode: add `--watch` to `xp2p client|server state` to stream tables with TTL filtering.
- Watch pending: combine `--watch --pending` to see staged tunnels while waiting for apply/service start.
- Custom diagnostics port/proto: `xp2p diag --listen 0.0.0.0:62025 --proto udp`.
- Custom ping port: `xp2p ping <host> --port 62025`.
- Tunnel cascade overrides: `xp2p ping <host> -T <target>`; use `-e <tag>` or `-i <index>` (with `-T`) when multiple endpoints share the same host.
- Access control: diagnostics port is intentionally unauthenticated; restrict it via firewall/ACL (for example allow only LAN and/or the tunnel interface).
  - OpenWrt (UCI): `uci add firewall rule; uci set firewall.@rule[-1].name='xp2p-diag'; uci set firewall.@rule[-1].src='lan'; uci set firewall.@rule[-1].proto='tcp'; uci set firewall.@rule[-1].dest_port='62022'; uci set firewall.@rule[-1].target='ACCEPT'; uci commit firewall; /etc/init.d/firewall restart`.
  - Linux (nftables): `nft add rule inet filter input tcp dport 62022 ip saddr { 127.0.0.1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } accept`.
  - Windows: `New-NetFirewallRule -DisplayName 'xp2p diagnostics' -Direction Inbound -Protocol TCP -LocalPort 62022 -Action Allow -RemoteAddress LocalSubnet`.
