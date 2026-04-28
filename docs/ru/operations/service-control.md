# Управление сервисами

Большинство установок управляет сервисами через менеджер сервисов ОС (`systemd`, `procd`, Windows SCM).
Команды `xp2p ... service` — это кроссплатформенный wrapper-слой.

## Status / stop

```sh
xp2p client service status
xp2p client service stop

xp2p server service status
xp2p server service stop
```

## Foreground run (для service managers)

`service run` предназначен для service managers, чтобы держать xp2p в foreground.
Используй это только если ты точно понимаешь, что тебе нужен foreground process вместо установки service unit.

```sh
xp2p client service run
xp2p server service run
```

## Foreground run (вручную)

```sh
xp2p client run
xp2p server run
```

