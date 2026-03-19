# Redirects Within A-B

Use this after the single A-B tunnel is working.

## Client redirect (CIDR)

On B (client), send a subnet through the tunnel:

```sh
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
```

## Client redirect (domain)

```sh
xp2p client redirect add --path /etc/xp2p --config-dir config-client --domain host.corp.test.com --tag proxy-10-63-30-11
```

## Server redirect (reverse)

On A (server), push traffic back through the tunnel:

```sh
xp2p server redirect add --path /etc/xp2p --config-dir config-server --cidr 10.0.102.0/24 --tag rev-user-10-63-30-11
```

## NAT redirect (proxy flow)

If you need transparent NAT redirect:

```sh
xp2p client nat-redirect add --cidr 10.0.101.0/24 --quiet
```

## DNS handling

### Tun with routing in place

If routes already send DNS traffic through the tunnel, use dnsmasq directly:

```
/corp.test.com/10.0.101.142#53
```

### Proxy or selective routing

Let xp2p manage a local forward and dnsmasq entry:

```sh
xp2p client dns-forward add -d corp.test.com -t 10.0.101.142:53 --with-forward
```
