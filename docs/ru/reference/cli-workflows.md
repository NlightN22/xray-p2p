# CLI сценарии

Эта страница — шпаргалка по командам. Для концептуальных деталей см. [Deploy flow](../flows/deploy-flow.md) и [Apply flow](../flows/apply-flow.md).

На Linux команды, которые меняют состояние системы, выполняй от root (используй `sudo`).

## Жизненный цикл сервера

Команды `xp2p server` управляют входящими слушателями Xray, TLS-артефактами и состоянием пользователей. Типичный набор действий:

```bash
xp2p server install --host edge.example.com
xp2p server service start

# Пользователи и reverse bridges
xp2p server user add --id branch@example.com --password S3cret
xp2p server user list
xp2p server user remove --id branch@example.com
xp2p server reverse list

# Сетевые хелперы
xp2p server redirect add --cidr 10.20.0.0/16
xp2p server redirect list
xp2p server redirect remove --cidr 10.20.0.0/16
xp2p server forward add --target 192.0.2.10:22
xp2p server forward list
xp2p server forward remove --target 192.0.2.10:22

# Только Linux/OpenWrt (интеграция с dnsmasq)
xp2p server dns-forward add --domain corp.example --target 10.10.10.53:53
xp2p server dns-forward list
xp2p server dns-forward remove --domain corp.example

# Обслуживание TLS
xp2p server cert set --cert /path/to/fullchain.pem --key /path/to/privkey.pem
xp2p server cert state
```

Подробнее о жизненном цикле сертификатов, включая замену, область удаления,
копирование и renewal hooks, см. [Server Certificates](../operations/server-certificates.md).

`dns-forward remove` удаляет управляемую dnsmasq-запись домена. Xray forward
удаляется только когда он был создан и всё ещё принадлежит dns-forward, и когда
ни один другой dns-forward domain не использует тот же listen port.

По умолчанию сервер работает в proxy mode (`server.tun_enabled = false`). Включай TUN явно через config или `XP2P_SERVER_TUN_ENABLED=true`, когда это нужно.

## Жизненный цикл клиента

Команды `xp2p client` настраивают роутеры OpenWrt, Linux-хосты или Windows-рабочие станции. Release-архивы кладут `xray` рядом с `xp2p`, поэтому держи оба бинарника вместе, если копируешь директорию установки между хостами.

```bash
# Установка из trojan:// link (автозаполняет user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"

xp2p client list
xp2p client service start

# Политики LAN
xp2p client redirect add --cidr 192.168.10.0/24
xp2p client redirect add --domain "*.corp.example"
xp2p client redirect remove --cidr 192.168.10.0/24
xp2p client redirect list

# Forwards и reverse tunnels
xp2p client forward add --target 192.0.2.10:22
xp2p client forward list
xp2p client forward remove --target 192.0.2.10:22
xp2p client reverse list

# DNS/DHCP helpers

# Только Linux/OpenWrt (интеграция с dnsmasq)
xp2p client dns-forward add --domain dev.example --target 10.10.10.53:53
xp2p client dns-forward list
xp2p client dns-forward remove --domain dev.example
```

`dns-forward remove` следует одному правилу владения для client и server roles:
существовавшие ранее forwards сохраняются, а общие dns-forward-owned forwards
остаются до удаления последнего домена, использующего тот же listen port.

Расширенные опции:

- Ручные поля клиента (без ссылки): `xp2p client install --host <host> --user <user> --password <password>`.
- Самоподписанный TLS: добавь `--allow-insecure` к `xp2p client install`.
- Выбор режима при установке: `xp2p client install --mode proxy|tun` (и `--tun-mode full|split`, если используешь TUN).
- Полное удаление: `xp2p client remove --all` удаляет конфигурацию клиента и бинарники.
