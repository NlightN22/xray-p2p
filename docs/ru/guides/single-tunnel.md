# Один туннель (A-B)

Этот раздел покрывает самый простой туннель A-B. Начни с deploy, а install используй,
если нужен явный контроль.

## Деплой (самый быстрый путь)

На B (client) сгенерируй deploy link:

```sh
xp2p client deploy --host 10.63.30.11
```

Скопируй ссылку, напечатанную в `client deploy: link generated`.

На A (server) запусти deploy со ссылкой:

```sh
xp2p server deploy --link "<PASTE_LINK>"
```

По умолчанию deploy включает TUN mode со split routing на клиенте. Используй `--mode proxy`, если хочешь оставить клиент в proxy mode.

### Расширенные опции деплоя

Оставить клиент в proxy mode после deploy:

```sh
xp2p client deploy --host 10.63.30.11 --mode proxy
```

Deploy сразу в full-tunnel TUN mode:

```sh
xp2p client deploy --host 10.63.30.11 --mode tun:full
```

Кастомный deploy port (передай его на обеих сторонах):

```sh
xp2p client deploy --host 10.63.30.11 --port 62125
xp2p server deploy --listen :62125 --link "<PASTE_LINK>"
```

## Установка (ручной путь)

На A (server):

```sh
xp2p server install --host 10.63.30.11
```

На B (client) используй ссылку из вывода server install:

```sh
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
```

## Применение (запуск сервисов)

И deploy, и install обновляют Desired inputs и пишут `apply.request`. Запусти (или перезапусти) сервисы, чтобы применить изменения:

```sh
xp2p server service start
xp2p client service start
```

На Linux команды, которые меняют состояние системы, выполняй от root (используй `sudo`).

## Переключение режимов (после установки/деплоя)

После установки туннеля можно переключать режимы без повторного deploy/install. Команды обновляют Desired inputs, записывают apply request и автоматически перезапустят сервис, если он запущен.

Переключить клиента в proxy mode (отключает TUN):

```sh
xp2p client mode proxy
```

Переключить клиента в split-tunnel TUN mode:

```sh
xp2p client mode tun split
```

Переключить клиента в full-tunnel TUN mode:

```sh
xp2p client mode tun full
```

Переключить режим сервера:

```sh
xp2p server mode proxy
xp2p server mode tun
```

## Проверка

С B:

```sh
xp2p ping 10.63.30.11
```

Для `xp2p ping` нужен diagnostics responder на целевой стороне (service xp2p или `xp2p diag`).

Advanced verification options:

- Используй `--tunnel`, чтобы принудительно пустить ping через туннель, и `--count <n>`, чтобы ограничить число запросов.
