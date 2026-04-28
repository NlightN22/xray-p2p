# CLI сценарии

Эта страница — command-oriented cheat sheet. Для концептуальных деталей см. [Deploy flow](../flows/deploy-flow.md) и [Apply flow](../flows/apply-flow.md).

На Linux команды, которые меняют состояние системы, выполняй от root (используй `sudo`).

## Жизненный цикл сервера

Server команды управляют xray inbound listeners, TLS assets и user state. Типичный flow выглядит так:

```bash
xp2p server install --host edge.example.com
xp2p server service start

# Users и reverse bridges
xp2p server user add --id branch@example.com --password S3cret
xp2p server user list
xp2p server user remove --id branch@example.com
xp2p server reverse list

# Networking helpers
xp2p server redirect add --cidr 10.20.0.0/16
xp2p server redirect list
xp2p server redirect remove --cidr 10.20.0.0/16
xp2p server forward add --target 192.0.2.10:22
xp2p server forward list
xp2p server forward remove --target 192.0.2.10:22

# Linux/OpenWrt only (dnsmasq integration)
xp2p server dns-forward add --domain corp.example --target 10.10.10.53:53
xp2p server dns-forward list
xp2p server dns-forward remove --domain corp.example

# TLS upkeep
xp2p server cert set --cert /path/to/fullchain.pem --key /path/to/privkey.pem
xp2p server cert state
```

По умолчанию server работает в proxy mode (`server.tun_enabled = false`). Включай TUN явно через config или `XP2P_SERVER_TUN_ENABLED=true`, когда это нужно.

## Жизненный цикл клиента

Client команды настраивают OpenWrt роутеры, Linux hosts или Windows workstations. Release archives уже кладут `xray` рядом с `xp2p`, поэтому держи оба бинарника вместе, если копируешь install directory между хостами.

```bash
# Install из trojan:// link (автозаполняет user, host, password, TLS settings)
xp2p client install --link "trojan://PASSWORD@edge.example.com:62022?security=tls#office@example.com"

xp2p client list
xp2p client service start

# LAN policy helpers
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

# Linux/OpenWrt only (dnsmasq integration)
xp2p client dns-forward add --domain dev.example --target 10.10.10.53:53
xp2p client dns-forward list
xp2p client dns-forward remove --domain dev.example
```

Advanced options:

- Manual client fields (no link): `xp2p client install --host <host> --user <user> --password <password>`.
- Self-signed TLS: добавь `--allow-insecure` к `xp2p client install`.
- Выбор режима при install: `xp2p client install --mode proxy|tun` (и `--tun-mode full|split`, если используешь TUN).
- Полное удаление: `xp2p client remove --all` удаляет client configuration и binaries.

