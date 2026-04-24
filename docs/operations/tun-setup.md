# TUN setup

TUN interfaces default to `xp2pc` (client) and `xp2ps` (server). When you change the names, MTU values, or addresses, update the OS network configuration and re-run `xp2p {client,server} install`.

## OpenWrt

On OpenWrt, `xp2p` provisions the UCI network interface for you when TUN is enabled and removes it on `xp2p {client,server} remove`. Use the commands below only when you need manual overrides or recovery.

Client example (manual override):

```sh
uci -q delete network.xp2pc
uci set network.xp2pc='interface'
uci set network.xp2pc.device='xp2pc'
uci set network.xp2pc.proto='static'
uci add_list network.xp2pc.ipaddr='198.18.0.1/30'
uci set network.xp2pc.xp2p_managed='1'
uci commit network
/etc/init.d/network reload
ip a show dev xp2pc
```

Server example (manual override):

```sh
uci -q delete network.xp2ps
uci set network.xp2ps='interface'
uci set network.xp2ps.device='xp2ps'
uci set network.xp2ps.proto='static'
uci add_list network.xp2ps.ipaddr='198.18.0.5/30'
uci set network.xp2ps.xp2p_managed='1'
uci commit network
/etc/init.d/network reload
ip a show dev xp2ps
```

## Linux (routing)

On Linux, `xp2p` configures TUN routing at runtime using `ip rule` and `ip route`. The default policy route table is `20090` for `xp2pc` and `20091` for `xp2ps`.

## Windows

Place `wintun.dll` next to `xp2p.exe` and `xray.exe` (for MSI installs this is the `bin` directory). The TUN interface is created automatically when Xray starts.
