# Chain (C2-B-A-C1)

This chain sends traffic from C2 through B and A to reach C1.

## Assumptions

- A = server, B = client.
- C1 behind A (10.0.101.0/24), C2 behind B (10.0.102.0/24).
- A-B tunnel is already working.

## Routes on C1 and C2

On C1:

```sh
ip route add 10.0.102.0/24 via 10.0.101.1
```

On C2:

```sh
ip route add 10.0.101.0/24 via 10.0.102.1
```

## Redirect on B

```sh
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
```

## OpenWrt firewall on B

Bind the xp2pc interface to a firewall zone and allow LAN <-> xp2ptun forwarding:

```sh
uci -q delete firewall.xp2ptun
uci set firewall.xp2ptun='zone'
uci set firewall.xp2ptun.name='xp2ptun'
uci set firewall.xp2ptun.network='xp2pc'
uci set firewall.xp2ptun.input='ACCEPT'
uci set firewall.xp2ptun.output='ACCEPT'
uci set firewall.xp2ptun.forward='ACCEPT'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='lan'
uci set firewall.@forwarding[-1].dest='xp2ptun'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='xp2ptun'
uci set firewall.@forwarding[-1].dest='lan'

uci commit firewall
/etc/init.d/firewall restart
```

## Verify

From C2:

```sh
ping -c 1 10.0.101.1
```
