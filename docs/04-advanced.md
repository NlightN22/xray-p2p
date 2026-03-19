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
