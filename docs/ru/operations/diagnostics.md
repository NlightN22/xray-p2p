# Диагностика и NAT-хелперы

## Общее

- Состояние (heartbeat/state): `xp2p client state` и `xp2p server state`.
- Просмотр pending: добавь `--pending`, чтобы увидеть настроенные туннели до того, как сервис применит изменения.
- Ответчик диагностики: `xp2p diag` запускает слушатель в foreground для `xp2p ping`.
- Форвардинг: `xp2p client forward add|list|remove` и `xp2p server forward add|list|remove`.
- DNS/DHCP (только Linux/OpenWrt): `xp2p {client,server} dns-forward add|remove|list`.
- NAT snippets (только Linux/OpenWrt): `xp2p nat-redirect add --cidr 192.168.10.0/24` генерирует фрагменты правил для прозрачного перехвата.

## Проверки ping

`xp2p ping` — это проверка связности на уровне протокола для ответчика диагностики.
Она работает, когда на удалённой стороне запущен xp2p (сервис клиента/сервера) или когда ты запускаешь `xp2p diag` на этой ноде.

Если ты не используешь этот проект end-to-end, `xp2p ping` всё равно может работать, если на ноде, где развёрнут Xray (или на любой ноде, которую ты хочешь проверять), запущен `xp2p diag`.

Прямой ping (без туннеля, по умолчанию TCP/62022):

```sh
xp2p ping <host>
```

Пример (проверить ноду по `host`):

```sh
xp2p ping edge.example.com
```

Reverse/tunnel ping (через SOCKS-туннель xp2p):

```sh
xp2p ping <host> --tunnel
```

Пример (проверить через туннель клиента по `host`):

```sh
xp2p ping edge.example.com --tunnel
```

Пример (проверить reverse-канал, передав reverse tag как аргумент `<host>`):

```sh
xp2p ping reverse-alpha.rev --tunnel
```

Также можно выбрать reverse-канал по user id (если он однозначно соответствует одному reverse portal):

```sh
xp2p ping deploy-1777353786@local --tunnel
```

Где смотреть значения `host`/`tag`:

- Эндпоинты клиента: `xp2p client list` печатает `host` и `tag` для настроенных эндпоинтов.
- Reverse-каналы сервера: `xp2p server reverse list` печатает reverse `tag`, `host` и `user`.
- Пользователи сервера: `xp2p server user list` выводит user ids, которые лежат в основе reverse portals (создаются при `xp2p server user add`, если не отключено).
- Таблица heartbeat сервера: `xp2p server state` печатает колонку `CLIENT_USER` для live туннелей.

Если несколько эндпоинтов используют один и тот же `host`, используй селектор:

```sh
xp2p ping edge.example.com --tunnel --endpoint proxy-edge
xp2p ping edge.example.com --tunnel --index 2
```

При использовании tunnel mode xp2p может маршрутизировать probe через internal marker target. Для reverse-каналов marker port отличается (62023) и выбирается автоматически.

## Расширенные опции / устранение неполадок

- Режим наблюдения: добавь `--watch` к `xp2p client|server state`, чтобы стримить таблицы с TTL filtering.
- Наблюдение pending: комбинируй `--watch --pending`, чтобы видеть staged туннели в ожидании apply/service start.
- Кастомный порт/протокол диагностики: `xp2p diag --listen 0.0.0.0:62025 --proto udp`.
- Кастомный ping port: `xp2p ping <host> --port 62025`.
- Переопределение маршрута через туннель: `xp2p ping <host> -T <target>`; используй `-e <tag>` или `-i <index>` (вместе с `-T`), когда несколько эндпоинтов используют один `host`.
- Контроль доступа: порт диагностики намеренно без аутентификации; ограничь его через firewall/ACL (например разреши только LAN и/или туннельный интерфейс).
  - OpenWrt (UCI): `uci add firewall rule; uci set firewall.@rule[-1].name='xp2p-diag'; uci set firewall.@rule[-1].src='lan'; uci set firewall.@rule[-1].proto='tcp'; uci set firewall.@rule[-1].dest_port='62022'; uci set firewall.@rule[-1].target='ACCEPT'; uci commit firewall; /etc/init.d/firewall restart`.
  - Linux (nftables): `nft add rule inet filter input tcp dport 62022 ip saddr { 127.0.0.1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } accept`.
  - Windows: `New-NetFirewallRule -DisplayName 'xp2p diagnostics' -Direction Inbound -Protocol TCP -LocalPort 62022 -Action Allow -RemoteAddress LocalSubnet`.
