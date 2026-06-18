# Установка

Эта страница описывает базовую установку по платформам. Для самого быстрого end-to-end сценария начни с [First tunnel (A-B)](../guides/single-tunnel.md).

## OpenWrt

Однострочный установщик (автоопределяет релиз/архитектуру, добавляет feed/ключ подписи, устанавливает пакет):

```console
wget -qO- https://nlightn22.github.io/xray-p2p/install-xp2p.sh | sh
```

- Сервисы устанавливаются как `/etc/init.d/xp2p-client` и `/etc/init.d/xp2p-server` и запускают `xp2p client|server service run` с параметрами по умолчанию.
- Управление: `service xp2p-client start|stop|restart|status` или `xp2p client service start|status`; логи лежат в `/var/log/xp2p/`.
- На живой системе OpenWrt `opkg install` вызывает `default_postinst`, который сразу включает и запускает init.d сервисы.
- Удаление: `opkg remove xp2p` (останавливает сервисы и удаляет пакет). Для полной очистки удалите `/etc/xp2p`, `/var/log/xp2p` и init-скрипты.

Опциональная ручная настройка feed:

```console
echo "src-git xp2p https://github.com/NlightN22/xray-p2p.git;main" >> /etc/opkg/customfeeds.conf && opkg update && opkg install xp2p
```

Установка из локального IPK:

```console
opkg install /tmp/xp2p_<version>_<arch>.ipk
```

## Linux (пакеты Debian/Ubuntu)

- Скачайте `.deb` под вашу архитектуру из Release (`xp2p_<version>_amd64.deb`, `xp2p_<version>_arm64.deb`, `xp2p_<version>_armhf.deb`, `xp2p_<version>_386.deb`).
- Установка: `sudo dpkg -i xp2p_<version>_<arch>.deb || sudo apt-get -f install`.
- Бинарники: `/usr/bin/xp2p`, bundled `xray`: `/etc/xp2p/bin/xray`, конфиги: `/etc/xp2p/config-{client,server}`, логи: `/var/log/xp2p`.
- Сервисы: `systemctl enable --now xp2p-client xp2p-server` (обёртки над `xp2p client|server service run` с параметрами по умолчанию).
- Удаление: `sudo dpkg -r xp2p`; очистка данных: `sudo dpkg -P xp2p`.

## Windows (MSI)

- Скачайте `xp2p-<version>-windows-amd64.msi` (или `.zip` архив).
- Установка MSI стандартными командами:

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
msiexec /x xp2p-<version>-windows-amd64.msi
```

- Сервисы `xp2p-client` и `xp2p-server` оборачивают `xp2p client|server service run`; управлять можно через `xp2p client service start|stop|status` или оснастку Services.
- Логи: `C:\Program Files\xp2p\logs\<role>\`.
- Управление сервисами (start/stop/status) доступно Builtin Users без admin elevation.
- Трей-контроллер `ui-xp2p` ставится вместе с MSI и автозапускается через Run key текущего пользователя; отключение: `XP2P_UI_AUTOSTART=0`.

Нужно собрать Windows MSI на Windows-хосте? См. [`scripts/build/README.md`](https://github.com/NlightN22/xray-p2p/blob/main/scripts/build/README.md).

## Архивы (tar.gz релизы)

Release-архивы содержат `xp2p` вместе с bundled `xray`. Держите оба бинарника рядом и добавьте `xp2p` в `PATH`.

## Быстрый старт (ручная установка)

В этих примерах используется `xp2p server install` + `xp2p client install` (без deploy handshake). Для пути через deploy handshake см. [First tunnel (A-B)](../guides/single-tunnel.md).

### OpenWrt

```console
opkg update && opkg install xp2p
xp2p server install --host edge.example.com
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
xp2p server service start
xp2p client service start
xp2p server state
```

### Linux

```console
sudo dpkg -i xp2p_<version>_amd64.deb || sudo apt-get -f install
sudo xp2p server install --host edge.example.com
sudo xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
sudo systemctl enable --now xp2p-server xp2p-client
xp2p server state
```

### Windows

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
xp2p server install --host edge.example.com
xp2p client install --link "<LINK_FROM_SERVER_INSTALL>"
xp2p client service start
xp2p server service start
xp2p client reverse list
```

Advanced options:

- Custom service port: добавьте `--port <port>` в `xp2p server install` и используйте сгенерированную ссылку на клиенте.
- Self-signed TLS: передайте `--allow-insecure` в `xp2p client install`.
- Manual client fields (no link): `xp2p client install --host <host> --user <user> --password <password>`.
