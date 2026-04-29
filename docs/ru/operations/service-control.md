# Управление сервисами

Обычно сервисами управляет менеджер сервисов ОС (`systemd`, `procd`, Windows SCM).
Команды `xp2p ... service` — кроссплатформенная обёртка для управления сервисами xp2p.

## Статус / остановка

```sh
xp2p client service status
xp2p client service stop

xp2p server service status
xp2p server service stop
```

## Запуск в переднем плане (для менеджеров сервисов)

`service run` предназначен для менеджеров сервисов, чтобы держать xp2p в переднем плане.
Используй это только если осознанно запускаешь xp2p как foreground process вместо установки service unit.

```sh
xp2p client service run
xp2p server service run
```

## Запуск в переднем плане (вручную)

```sh
xp2p client run
xp2p server run
```

