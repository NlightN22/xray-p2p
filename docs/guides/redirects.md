# Redirects Within A-B

Use this after the single A-B tunnel is working.

## Client redirect (CIDR)

On B (client), send a subnet through the tunnel:

```sh
xp2p client redirect add --cidr 10.0.101.0/24
```

## Client redirect (domain)

```sh
xp2p client redirect add --domain host.corp.test.com
```

## Server redirect (reverse)

On A (server), push traffic back through the tunnel:

```sh
xp2p server redirect add --cidr 10.0.102.0/24
```

## NAT redirect (proxy flow)

If you need transparent NAT redirect (Linux/OpenWrt only, proxy mode only, run as root):

```sh
xp2p nat-redirect add --cidr 10.0.101.0/24
```

## DNS handling

### Tun with routing in place

If routes already send DNS traffic through the tunnel, use dnsmasq directly:

```
/corp.test.com/10.0.101.142#53
```

### Proxy or selective routing

Let xp2p manage a dnsmasq entry (and optionally a local forward):

```sh
xp2p client dns-forward add --domain corp.test.com --target 10.0.101.142:53
```

## Advanced options

- Multiple tunnels/endpoints: pass `--tag <tag>` to target a specific tunnel.
- Non-default config root: pass `--path <dir>` and `--config-dir <dir>` when you do not use the default layout.
- DNS helper forward: add `--with-forward` to also create the local forward rule alongside the dnsmasq entry.
