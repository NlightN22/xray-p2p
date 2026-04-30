# Расширенные варианты

Используй это, когда базовый A-B и сценарий chain уже работают.

## Несколько клиентов (B и C)

- Установи несколько клиентов на разных узлах OpenWrt.
- Используй разные config dirs для каждого клиента, чтобы избежать конфликтов.

```console
xp2p client install --path /etc/xp2p --config-dir config-client-b --link "<LINK_B>" --force
xp2p client install --path /etc/xp2p --config-dir config-client-c --link "<LINK_C>" --force
```

## Разделение маршрутизации по CIDR

```console
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.102.0/24 --tag proxy-10-63-30-12
```

## Режим full-tunnel

Full-tunnel mode доступен только когда клиент работает в TUN mode (`client.tun_enabled = true`).
Он заменяет default routes на TUN интерфейс, добавляет bypass routes ко всем настроенным endpoints
и переключает DNS resolvers на `client.dns_servers` на время активности full-tunnel.

Переключить через CLI:

```console
xp2p client mode tun full
```

Вернуться на split-tunnel:

```console
xp2p client mode tun split
```

Вернуться в proxy mode:

```console
xp2p client mode proxy
```

```toml
[client]
tun_enabled = true
tun_mode = "full"
dns_servers = ["1.1.1.1", "8.8.8.8"]
```

### Windows Server 2016

На Windows Server 2016 Wintun-адаптер может периодически оставаться disconnected после рестартов (IPv4 остаётся `Tentative`, routes не применяются). В этом случае `xp2p` держит смену режима в pending и повторяет попытки через рестарты сервиса, пока адаптер не станет `up`/`preferred`. Cleanup выполняется перед каждым стартом. Следующие сообщения xray ожидаемы при пересоздании адаптера: `Failed to find matching adapter name`, `Removed orphaned adapter`.

#### Контракт стабильности full-tunnel

Full-tunnel — это runtime mode сервиса и он должен оставаться активным, пока Desired mode — full-tunnel.

- Рестарты сервиса из-за apply/watchers не должны откатывать routes или DNS, если Desired остаётся `tun_mode=full`.
- Когда адаптер не готов (`Tentative` / disconnected / отсутствует IPv4), runtime держит full-tunnel в pending и повторяет bring-up адаптера между рестартами (с rate limits).
- Routes и DNS override должны применяться только после того, как адаптер сообщает `up`/`preferred`, чтобы избежать connectivity flapping.

##### Задержка повторов в pending

Когда full-tunnel является Desired, но адаптер не готов, runtime входит в `FullPending` и логирует:

- `full-tunnel pending; deferring route apply until restart`

Повторы используют exponential backoff с максимумом 30 секунд (начиная с 2 секунд). Pending state и расписание повторов сохраняются в `CONFIG_ROOT/xp2p-client.tun-full.json` (`phase`, `pending_reason`, `retry_count`, `next_retry_at`), чтобы рестарты следовали одному контракту.

## DNS маршрутизация по доменам (только Linux/OpenWrt)

```console
xp2p client dns-forward add -d corp.test.com -t 10.0.101.142:53 --with-forward
xp2p client dns-forward add -d lab.test.com -t 10.0.102.142:53 --with-forward
```

## Очистка

```console
xp2p client redirect remove --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client dns-forward remove -d corp.test.com --with-forward
xp2p client remove --path /etc/xp2p --config-dir config-client --all --ignore-missing --quiet
```
