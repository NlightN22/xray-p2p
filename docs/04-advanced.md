# Advanced Variants

Use these when the basic A-B and chain scenarios are working.

## Multiple clients (B and C)

- Install multiple clients on different OpenWrt nodes.
- Keep per-client config dirs to avoid clashes.

```sh
xp2p client install --path /etc/xp2p --config-dir config-client-b --link "<LINK_B>" --force
xp2p client install --path /etc/xp2p --config-dir config-client-c --link "<LINK_C>" --force
```

## Split routing by CIDR

```sh
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.102.0/24 --tag proxy-10-63-30-12
```

## Full-tunnel mode

Full-tunnel mode is available only when the client runs in TUN mode (`client.tun_enabled = true`).
It replaces default routes with the TUN interface, adds bypass routes to all configured endpoints,
and switches DNS resolvers to `client.dns_servers` while full-tunnel is active.

```toml
[client]
tun_enabled = true
tun_mode = "full"
dns_servers = ["1.1.1.1", "8.8.8.8"]
```

### Windows Server 2016

On Windows Server 2016, the Wintun adapter can intermittently stay disconnected after restarts (IPv4 remains `Tentative`, routes do not apply). When this happens, `xp2p` keeps the mode change pending and retries through service restarts until the adapter reports `up`/`preferred`. Cleanup runs before each start. The following xray logs are expected during adapter recreation: `Failed to find matching adapter name`, `Removed orphaned adapter`.

## DNS per-domain routing

```sh
xp2p client dns-forward add -d corp.test.com -t 10.0.101.142:53 --with-forward
xp2p client dns-forward add -d lab.test.com -t 10.0.102.142:53 --with-forward
```

## Cleanup

```sh
xp2p client redirect remove --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client dns-forward remove -d corp.test.com --with-forward
xp2p client remove --path /etc/xp2p --config-dir config-client --all --ignore-missing --quiet
```
