# Редиректы в A-B

Используй это после того, как базовый A-B туннель уже работает.

## Редирект клиента (CIDR)

На B (client) отправь сеть через туннель:

```console
xp2p client redirect add --cidr 10.0.101.0/24
```

Когда TUN включён, это также установит OS route для CIDR (если не использовать `--no-routes`).

## Редирект клиента (домен)

```console
xp2p client redirect add --domain host.corp.test.com
```

## Редирект сервера (реверс)

На A (server) отправь трафик обратно через туннель:

```console
xp2p server redirect add --cidr 10.0.102.0/24
```

Когда TUN включён, это также установит OS route для CIDR (если не использовать `--no-routes`).

## NAT редирект (прокси-режим)

Если нужен прозрачный NAT redirect (только Linux/OpenWrt, только proxy mode, выполнять от root):

```console
xp2p nat-redirect add --cidr 10.0.101.0/24
```

Просмотр и удаление NAT redirect правил:

```console
xp2p nat-redirect list
xp2p nat-redirect remove --cidr 10.0.101.0/24
```

## DNS

### TUN с уже настроенной маршрутизацией

Если routes уже отправляют DNS-трафик через туннель, используй dnsmasq напрямую:

```
/corp.test.com/10.0.101.142#53
```

### Прокси или селективная маршрутизация

Пусть xp2p управляет записью dnsmasq (и, опционально, локальным forward) (только Linux/OpenWrt):

```console
xp2p client dns-forward add --domain corp.test.com --target 10.0.101.142:53
```

Просмотр и удаление правил:

```console
xp2p client redirect list
xp2p client forward remove --target 192.0.2.10:22
xp2p server redirect list
xp2p server redirect remove --cidr 10.0.102.0/24
xp2p server forward list
xp2p server forward remove --target 192.0.2.10:22
xp2p server dns-forward remove --domain corp.test.com
```

## Расширенные опции

- Несколько tunnels/endpoints: передай `--tag <tag>`, чтобы выбрать конкретный туннель.
- Нестандартный корень конфигурации: передай `--path <dir>` и `--config-dir <dir>`, если не используешь layout по умолчанию.
- DNS helper forward: добавь `--with-forward`, чтобы одновременно создать локальный forward-правило вместе с dnsmasq записью.
