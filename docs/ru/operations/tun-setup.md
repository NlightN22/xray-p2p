# РќР°СЃС‚СЂРѕР№РєР° TUN

TUN-РёРЅС‚РµСЂС„РµР№СЃС‹ РёСЃРїРѕР»СЊР·СѓСЋС‚СЃСЏ `xp2pc` (client) Рё `xp2ps` (server). РћР±С‹С‡РЅРѕ Р°РґСЂРµСЃР°, MTU Рё Р±Р°Р·РѕРІС‹Рµ РїР°СЂР°РјРµС‚СЂС‹ TUN Р·Р°РґР°СЋС‚СЃСЏ С‡РµСЂРµР· `xp2p {client,server} install`.

## РџСЂРµРґРІР°СЂРёС‚РµР»СЊРЅС‹Рµ С‚СЂРµР±РѕРІР°РЅРёСЏ

РљРѕРіРґР° РІРєР»СЋС‡С‘РЅ TUN, `xp2p` РІС‹РїРѕР»РЅСЏРµС‚ preflight-РїСЂРѕРІРµСЂРєСѓ РІ СЂР°РЅС‚Р°Р№РјРµ Рё Р·Р°РІРµСЂС€Р°РµС‚ СЂР°Р±РѕС‚Сѓ СЂР°РЅСЊС€Рµ (РґРѕ СЃС‚Р°СЂС‚Р° Xray), РµСЃР»Рё РЅРµРѕР±С…РѕРґРёРјС‹Рµ Р·Р°РІРёСЃРёРјРѕСЃС‚Рё РѕС‚СЃСѓС‚СЃС‚РІСѓСЋС‚.

- OpenWrt: РЅСѓР¶РµРЅ РїР°РєРµС‚ `kmod-tun` (С‡С‚РѕР±С‹ РїРѕСЏРІРёР»СЃСЏ `/dev/net/tun`).
- Linux: РЅСѓР¶РµРЅ `/dev/net/tun` (`modprobe tun`) Рё РїСЂР°РІР° РЅР° СѓРїСЂР°РІР»РµРЅРёРµ СЃРµС‚СЊСЋ (root РёР»Рё `CAP_NET_ADMIN`).
- Windows: `wintun.dll` РґРѕР»Р¶РµРЅ Р»РµР¶Р°С‚СЊ СЂСЏРґРѕРј СЃ `xp2p.exe` Рё `xray.exe` (РІ MSI-СѓСЃС‚Р°РЅРѕРІРєРµ РѕР±С‹С‡РЅРѕ СЌС‚Рѕ `<install_dir>/bin`).

## OpenWrt

РќР° OpenWrt `xp2p` СЃРѕР·РґР°С‘С‚ UCI СЃРµС‚РµРІРѕР№ РёРЅС‚РµСЂС„РµР№СЃ Рё СѓРїСЂР°РІР»СЏРµС‚ РёРј РїСЂРё СѓСЃС‚Р°РЅРѕРІРєРµ/СѓРґР°Р»РµРЅРёРё СЂРѕР»Рё. Р•СЃР»Рё РЅСѓР¶РЅРѕ РІСЂСѓС‡РЅСѓСЋ РїРµСЂРµРѕРїСЂРµРґРµР»РёС‚СЊ РёРЅС‚РµСЂС„РµР№СЃ, РјРѕР¶РЅРѕ Р·Р°РґР°С‚СЊ РµРіРѕ РІ UCI.

Р СѓС‡РЅРѕРµ РїРµСЂРµРѕРїСЂРµРґРµР»РµРЅРёРµ РґР»СЏ client:

```console
uci -q delete network.xp2pc
uci set network.xp2pc='interface'
uci set network.xp2pc.device='xp2pc'
uci set network.xp2pc.proto='static'
uci add_list network.xp2pc.ipaddr='198.18.0.1/30'
uci set network.xp2pc.xp2p_managed='1'
uci commit network
/etc/init.d/network reload
ip a show dev xp2pc
```

Р СѓС‡РЅРѕРµ РїРµСЂРµРѕРїСЂРµРґРµР»РµРЅРёРµ РґР»СЏ server:

```console
uci -q delete network.xp2ps
uci set network.xp2ps='interface'
uci set network.xp2ps.device='xp2ps'
uci set network.xp2ps.proto='static'
uci add_list network.xp2ps.ipaddr='198.18.0.5/30'
uci set network.xp2ps.xp2p_managed='1'
uci commit network
/etc/init.d/network reload
ip a show dev xp2ps
```

## Linux (РјР°СЂС€СЂСѓС‚РёР·Р°С†РёСЏ policy routing)

РќР° Linux `xp2p` РЅР°СЃС‚СЂР°РёРІР°РµС‚ РјР°СЂС€СЂСѓС‚РёР·Р°С†РёСЋ TUN РІ СЂР°РЅС‚Р°Р№РјРµ СЃ РїРѕРјРѕС‰СЊСЋ `ip rule` Рё `ip route`.

РўР°Р±Р»РёС†Р° РјР°СЂС€СЂСѓС‚РёР·Р°С†РёРё policy routing РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ: `20090` РґР»СЏ `xp2pc` Рё `20091` РґР»СЏ `xp2ps`.

## Windows

РќР° Windows РґР»СЏ TUN РёСЃРїРѕР»СЊР·СѓРµС‚СЃСЏ Wintun. Р›РѕРіРё xray-core РІСЂРѕРґРµ `Failed to find matching adapter name` Рё `Removed orphaned adapter` РѕР¶РёРґР°РµРјС‹ РІРѕ РІСЂРµРјСЏ РїРѕРґРЅСЏС‚РёСЏ Wintun Рё СЃР°РјРё РїРѕ СЃРµР±Рµ РЅРµ СЏРІР»СЏСЋС‚СЃСЏ РѕС€РёР±РєРѕР№.
