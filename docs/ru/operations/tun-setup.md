# Настройка TUN

TUN-интерфейсы используются `xp2pc` (client) и `xp2ps` (server). Обычно адреса, MTU и базовые параметры TUN задаются через `xp2p {client,server} install`.

## Предварительные требования

Когда включён TUN, `xp2p` выполняет preflight-проверку в рантайме и завершает работу раньше (до старта Xray), если необходимые зависимости отсутствуют.

- OpenWrt: нужен пакет `kmod-tun` (чтобы появился `/dev/net/tun`).
- Linux: нужен `/dev/net/tun` (`modprobe tun`) и права на управление сетью (root или `CAP_NET_ADMIN`).
- Windows: `wintun.dll` должен лежать рядом с `xp2p.exe` и `xray.exe` (в MSI-установке обычно это `<install_dir>/bin`).

## OpenWrt

На OpenWrt `xp2p` создаёт UCI сетевой интерфейс и управляет им при установке/удалении роли. Если нужно вручную переопределить интерфейс, можно задать его в UCI.

Ручное переопределение для client:

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

Ручное переопределение для server:

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

## Linux (маршрутизация policy routing)

На Linux `xp2p` настраивает маршрутизацию TUN в рантайме с помощью `ip rule` и `ip route`.

Таблица маршрутизации policy routing по умолчанию: `20090` для `xp2pc` и `20091` для `xp2ps`.

## Windows

На Windows для TUN используется Wintun. Логи xray-core вроде `Failed to find matching adapter name` и `Removed orphaned adapter` ожидаемы во время поднятия Wintun и сами по себе не являются ошибкой.
