# Service control

Most installations manage services via the OS service manager (`systemd`, `procd`, Windows SCM).
The `xp2p ... service` commands are the cross-platform wrapper layer.

## Status / stop

```console
xp2p client service status
xp2p client service stop

xp2p server service status
xp2p server service stop
```

## Foreground run (for service managers)

`service run` is intended for service managers to keep xp2p in the foreground.
Use it only when you know you want a foreground process instead of installing a service unit.

```console
xp2p client service run
xp2p server service run
```

## Foreground run (manual)

```console
xp2p client run
xp2p server run
```

