# РЈСЃС‚Р°РЅРѕРІРєР°

Р­С‚Р° СЃС‚СЂР°РЅРёС†Р° РѕРїРёСЃС‹РІР°РµС‚ Р±Р°Р·РѕРІСѓСЋ СѓСЃС‚Р°РЅРѕРІРєСѓ РїРѕ РїР»Р°С‚С„РѕСЂРјР°Рј. Р”Р»СЏ СЃР°РјРѕРіРѕ Р±С‹СЃС‚СЂРѕРіРѕ end-to-end СЃС†РµРЅР°СЂРёСЏ РЅР°С‡РЅРё СЃ [First tunnel (A-B)](../guides/single-tunnel.md).

## OpenWrt

РћРґРЅРѕСЃС‚СЂРѕС‡РЅС‹Р№ СѓСЃС‚Р°РЅРѕРІС‰РёРє (Р°РІС‚РѕРѕРїСЂРµРґРµР»СЏРµС‚ СЂРµР»РёР·/Р°СЂС…РёС‚РµРєС‚СѓСЂСѓ, РґРѕР±Р°РІР»СЏРµС‚ feed/РєР»СЋС‡ РїРѕРґРїРёСЃРё, СѓСЃС‚Р°РЅР°РІР»РёРІР°РµС‚ РїР°РєРµС‚):

```console
wget -qO- https://nlightn22.github.io/xray-p2p/install-xp2p.sh | sh
```

- РЎРµСЂРІРёСЃС‹ СѓСЃС‚Р°РЅР°РІР»РёРІР°СЋС‚СЃСЏ РєР°Рє `/etc/init.d/xp2p-client` Рё `/etc/init.d/xp2p-server` Рё Р·Р°РїСѓСЃРєР°СЋС‚ `xp2p client|server service run` СЃ РїР°СЂР°РјРµС‚СЂР°РјРё РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ.
- РЈРїСЂР°РІР»РµРЅРёРµ: `service xp2p-client start|stop|restart|status` РёР»Рё `xp2p client service start|status`; Р»РѕРіРё Р»РµР¶Р°С‚ РІ `/var/log/xp2p/`.
- РќР° Р¶РёРІРѕР№ СЃРёСЃС‚РµРјРµ OpenWrt `opkg install` РІС‹Р·С‹РІР°РµС‚ `default_postinst`, РєРѕС‚РѕСЂС‹Р№ СЃСЂР°Р·Сѓ РІРєР»СЋС‡Р°РµС‚ Рё Р·Р°РїСѓСЃРєР°РµС‚ init.d СЃРµСЂРІРёСЃС‹.
- РЈРґР°Р»РµРЅРёРµ: `opkg remove xp2p` (РѕСЃС‚Р°РЅР°РІР»РёРІР°РµС‚ СЃРµСЂРІРёСЃС‹ Рё СѓРґР°Р»СЏРµС‚ РїР°РєРµС‚). Р”Р»СЏ РїРѕР»РЅРѕР№ РѕС‡РёСЃС‚РєРё СѓРґР°Р»РёС‚Рµ `/etc/xp2p`, `/var/log/xp2p` Рё init-СЃРєСЂРёРїС‚С‹.

РћРїС†РёРѕРЅР°Р»СЊРЅР°СЏ СЂСѓС‡РЅР°СЏ РЅР°СЃС‚СЂРѕР№РєР° feed:

```console
echo "src-git xp2p https://github.com/NlightN22/xray-p2p.git;main" >> /etc/opkg/customfeeds.conf && opkg update && opkg install xp2p
```

РЈСЃС‚Р°РЅРѕРІРєР° РёР· Р»РѕРєР°Р»СЊРЅРѕРіРѕ IPK:

```console
opkg install /tmp/xp2p_<version>_<arch>.ipk
```

## Linux (РїР°РєРµС‚С‹ Debian/Ubuntu)

- РЎРєР°С‡Р°Р№С‚Рµ `.deb` РїРѕРґ РІР°С€Сѓ Р°СЂС…РёС‚РµРєС‚СѓСЂСѓ РёР· Release (`xp2p_<version>_amd64.deb`, `xp2p_<version>_arm64.deb`, `xp2p_<version>_armhf.deb`, `xp2p_<version>_386.deb`).
- РЈСЃС‚Р°РЅРѕРІРєР°: `sudo dpkg -i xp2p_<version>_<arch>.deb || sudo apt-get -f install`.
- Р‘РёРЅР°СЂРЅРёРєРё: `/usr/bin/xp2p`, bundled `xray`: `/etc/xp2p/bin/xray`, РєРѕРЅС„РёРіРё: `/etc/xp2p/config-{client,server}`, Р»РѕРіРё: `/var/log/xp2p`.
- РЎРµСЂРІРёСЃС‹: `systemctl enable --now xp2p-client xp2p-server` (РѕР±С‘СЂС‚РєРё РЅР°Рґ `xp2p client|server service run` СЃ РїР°СЂР°РјРµС‚СЂР°РјРё РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ).
- РЈРґР°Р»РµРЅРёРµ: `sudo dpkg -r xp2p`; РѕС‡РёСЃС‚РєР° РґР°РЅРЅС‹С…: `sudo dpkg -P xp2p`.

## Windows (MSI)

- РЎРєР°С‡Р°Р№С‚Рµ `xp2p-<version>-windows-amd64.msi` (РёР»Рё `.zip` Р°СЂС…РёРІ).
- РЈСЃС‚Р°РЅРѕРІРєР° MSI СЃС‚Р°РЅРґР°СЂС‚РЅС‹РјРё РєРѕРјР°РЅРґР°РјРё:

```powershell
msiexec /i xp2p-<version>-windows-amd64.msi
msiexec /x xp2p-<version>-windows-amd64.msi
```

- РЎРµСЂРІРёСЃС‹ `xp2p-client` Рё `xp2p-server` РѕР±РѕСЂР°С‡РёРІР°СЋС‚ `xp2p client|server service run`; СѓРїСЂР°РІР»СЏС‚СЊ РјРѕР¶РЅРѕ С‡РµСЂРµР· `xp2p client service start|stop|status` РёР»Рё РѕСЃРЅР°СЃС‚РєСѓ Services.
- Р›РѕРіРё: `C:\Program Files\xp2p\logs\<role>\`.
- РЈРїСЂР°РІР»РµРЅРёРµ СЃРµСЂРІРёСЃР°РјРё (start/stop/status) РґРѕСЃС‚СѓРїРЅРѕ Builtin Users Р±РµР· admin elevation.
- РўСЂРµР№-РєРѕРЅС‚СЂРѕР»Р»РµСЂ `ui-xp2p` СЃС‚Р°РІРёС‚СЃСЏ РІРјРµСЃС‚Рµ СЃ MSI Рё Р°РІС‚РѕР·Р°РїСѓСЃРєР°РµС‚СЃСЏ С‡РµСЂРµР· Run key С‚РµРєСѓС‰РµРіРѕ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ; РѕС‚РєР»СЋС‡РµРЅРёРµ: `XP2P_UI_AUTOSTART=0`.

РќСѓР¶РЅРѕ СЃРѕР±СЂР°С‚СЊ Windows MSI РЅР° Windows-С…РѕСЃС‚Рµ? РЎРј. [`scripts/build/README.md`](https://github.com/NlightN22/xray-p2p/blob/main/scripts/build/README.md).

## РђСЂС…РёРІС‹ (tar.gz СЂРµР»РёР·С‹)

Release-Р°СЂС…РёРІС‹ СЃРѕРґРµСЂР¶Р°С‚ `xp2p` РІРјРµСЃС‚Рµ СЃ bundled `xray`. Р”РµСЂР¶РёС‚Рµ РѕР±Р° Р±РёРЅР°СЂРЅРёРєР° СЂСЏРґРѕРј Рё РґРѕР±Р°РІСЊС‚Рµ `xp2p` РІ `PATH`.

## Р‘С‹СЃС‚СЂС‹Р№ СЃС‚Р°СЂС‚ (СЂСѓС‡РЅР°СЏ СѓСЃС‚Р°РЅРѕРІРєР°)

Р’ СЌС‚РёС… РїСЂРёРјРµСЂР°С… РёСЃРїРѕР»СЊР·СѓРµС‚СЃСЏ `xp2p server install` + `xp2p client install` (Р±РµР· deploy handshake). Р”Р»СЏ РїСѓС‚Рё С‡РµСЂРµР· deploy handshake СЃРј. [First tunnel (A-B)](../guides/single-tunnel.md).

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

- Custom Trojan port: РґРѕР±Р°РІСЊС‚Рµ `--port <port>` РІ `xp2p server install` Рё РёСЃРїРѕР»СЊР·СѓР№С‚Рµ СЃРіРµРЅРµСЂРёСЂРѕРІР°РЅРЅСѓСЋ СЃСЃС‹Р»РєСѓ РЅР° РєР»РёРµРЅС‚Рµ.
- Self-signed TLS: РїРµСЂРµРґР°Р№С‚Рµ `--allow-insecure` РІ `xp2p client install`.
- Manual client fields (no link): `xp2p client install --host <host> --user <user> --password <password>`.
