# Настройка TUN

TUN интерфейсы по умолчанию называются `xp2pc` (client) и `xp2ps` (server). Если ты меняешь имена, MTU или адреса, обнови сетевую конфигурацию ОС и снова запусти `xp2p {client,server} install`.

## Предварительные требования

Когда TUN включён, `xp2p` выполняет runtime preflight check и завершает работу раньше (до старта Xray), если prerequisites отсутствуют.

- OpenWrt: установи kernel module командой `opkg update && opkg install kmod-tun` (убедись, что существует `/dev/net/tun`).
- Linux: убедись, что существует `/dev/net/tun` (`modprobe tun`) и запускай с достаточными правами (root или `CAP_NET_ADMIN`).
- Windows: положи `wintun.dll` рядом с `xray.exe` (обычно `<install_dir>/bin`) и используй совместимую версию.

## OpenWrt

На OpenWrt `xp2p` сам создаёт UCI network interface при включённом TUN и удаляет его при `xp2p {client,server} remove`. Используй команды ниже только для ручных override или восстановления.

Пример для client (manual override):

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

Пример для server (manual override):

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

## Linux (маршрутизация)

На Linux `xp2p` настраивает TUN маршрутизацию в runtime с помощью `ip rule` и `ip route`. Default policy route table: `20090` для `xp2pc` и `20091` для `xp2ps`.

## Windows

Положи `wintun.dll` рядом с `xp2p.exe` и `xray.exe` (для MSI installs это директория `bin`). TUN интерфейс создаётся автоматически при старте Xray.

