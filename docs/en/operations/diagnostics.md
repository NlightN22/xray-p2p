# Diagnostics & NAT helpers

## Common

- Heartbeat/state: `xp2p client state` and `xp2p server state`.
- Pending view: add `--pending` to show configured tunnels before the service applies changes.
- Standalone responder: `xp2p diag` starts a foreground HTTPS listener with public readiness and ping endpoints.
- Forwarding: `xp2p client forward add|list|remove` and `xp2p server forward add|list|remove`.
- DNS/DHCP (Linux/OpenWrt only): `xp2p {client,server} dns-forward add|remove|list`.
- NAT snippets (Linux/OpenWrt only): `xp2p nat-redirect add --cidr 192.168.10.0/24` generates transparent intercept snippets.

## Ping checks

`xp2p ping` uses one HTTPS protocol in every mode: `POST /control/v1/ping`
with a JSON nonce. A product client or server service authenticates this request
from published Live runtime metadata and fails closed when that metadata is
missing, invalid, or incomplete.

`xp2p diag` is a standalone composition of the same ping handler. It needs no
xp2p installation, Desired configuration, Live runtime, or pre-created
credentials. It publishes only `/control/v1/ready` and `/control/v1/ping`; its
ping is public and also accepts otherwise valid requests carrying unused
authentication headers. Restrict access to this listener with a firewall or ACL.
The standalone sidecar proves that the diagnostic responder is reachable; it
does not prove that a complete xp2p control plane is installed.

Direct ping (no tunnel, defaults to TCP/62022):

```console
xp2p ping <host>
```

Example (probe a node by its hostname):

```console
xp2p ping edge.example.com
```

Reverse/tunnel ping (through the xp2p SOCKS tunnel):

```console
xp2p ping <host> --tunnel
```

Example (probe through the client tunnel by hostname):

```console
xp2p ping edge.example.com --tunnel
```

Example (probe through a reverse channel using the reverse tag as the host argument):

```console
xp2p ping reverse-alpha.rev --tunnel
```

You can also select a reverse channel by user id (when it matches a single reverse portal):

```console
xp2p ping deploy-1777353786@local --tunnel
```

Where to find host/tag values:

- Client endpoints: `xp2p client list` prints `host` and `tag` for configured client endpoints.
- Server reverse channels: `xp2p server reverse list` prints reverse `tag`, `host`, and `user`.
- Server users: `xp2p server user list` lists user ids that back reverse portals (created during `xp2p server user add` unless disabled).
- Server heartbeat table: `xp2p server state` prints the `CLIENT_USER` column for live tunnels.

When multiple client endpoints share the same host, use a selector:

```console
xp2p ping edge.example.com --tunnel --endpoint proxy-edge
xp2p ping edge.example.com --tunnel --index 2
```

When tunnel mode is used, xp2p may route the probe through an internal marker target. For reverse channels the marker port is different (62023) and is selected automatically.

## Advanced / troubleshooting

Each client endpoint has a `heartbeat_mode` policy in Desired configuration:

- `required` is the default for endpoints installed or deployed by xp2p.
- `auto` discovers whether an imported or subscription endpoint supports the
  authenticated xp2p diagnostics and heartbeat exchange. An endpoint without
  that sidecar is reported as `not-detected`, not `unhealthy`.
- `disabled` suppresses heartbeat requests and is only set explicitly.

The state table separates the policy (`MODE`), check mechanism (`CHECK`), last
attempt, last complete success, and failure stage. A complete probe and report
changes capability to detected. After that, repeated failures can produce
`unhealthy`; a later complete success restores `healthy`. Failure stages are
`marker`, `probe`, `report`, and `persistence`.

Heartbeat timestamps are UTC observations. TTL evaluation accepts up to 30
seconds of future clock skew; timestamps farther in the future are rejected as
`clock_skew`. Existing pre-0.2.8 JSON remains readable with legacy TTL rules.

- Watch mode: add `--watch` to `xp2p client|server state` to stream tables with TTL filtering.
- Watch pending: combine `--watch --pending` to see staged tunnels while waiting for apply/service start.
- Custom diagnostics listener: `xp2p diag --listen 0.0.0.0:62025`.
- Custom ping port: `xp2p ping <host> --port 62025`.
- Tunnel cascade overrides: `xp2p ping <host> -T <target>`; use `-e <tag>` or `-i <index>` (with `-T`) when multiple endpoints share the same host.
- Access control: only the standalone `xp2p diag` listener is intentionally unauthenticated. Product service endpoints remain authenticated. Restrict the standalone listener via firewall/ACL (for example allow only LAN and/or the tunnel interface).
  - OpenWrt (UCI): `uci add firewall rule; uci set firewall.@rule[-1].name='xp2p-diag'; uci set firewall.@rule[-1].src='lan'; uci set firewall.@rule[-1].proto='tcp'; uci set firewall.@rule[-1].dest_port='62022'; uci set firewall.@rule[-1].target='ACCEPT'; uci commit firewall; /etc/init.d/firewall restart`.
  - Linux (nftables): `nft add rule inet filter input tcp dport 62022 ip saddr { 127.0.0.1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } accept`.
  - Windows: `New-NetFirewallRule -DisplayName 'xp2p diagnostics' -Direction Inbound -Protocol TCP -LocalPort 62022 -Action Allow -RemoteAddress LocalSubnet`.
