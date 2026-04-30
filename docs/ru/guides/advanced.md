# Р Р°СЃС€РёСЂРµРЅРЅС‹Рµ РІР°СЂРёР°РЅС‚С‹

РСЃРїРѕР»СЊР·СѓР№ СЌС‚Рѕ, РєРѕРіРґР° Р±Р°Р·РѕРІС‹Р№ A-B Рё СЃС†РµРЅР°СЂРёР№ chain СѓР¶Рµ СЂР°Р±РѕС‚Р°СЋС‚.

## РќРµСЃРєРѕР»СЊРєРѕ РєР»РёРµРЅС‚РѕРІ (B Рё C)

- РЈСЃС‚Р°РЅРѕРІРё РЅРµСЃРєРѕР»СЊРєРѕ РєР»РёРµРЅС‚РѕРІ РЅР° СЂР°Р·РЅС‹С… СѓР·Р»Р°С… OpenWrt.
- РСЃРїРѕР»СЊР·СѓР№ СЂР°Р·РЅС‹Рµ config dirs РґР»СЏ РєР°Р¶РґРѕРіРѕ РєР»РёРµРЅС‚Р°, С‡С‚РѕР±С‹ РёР·Р±РµР¶Р°С‚СЊ РєРѕРЅС„Р»РёРєС‚РѕРІ.

```console
xp2p client install --path /etc/xp2p --config-dir config-client-b --link "<LINK_B>" --force
xp2p client install --path /etc/xp2p --config-dir config-client-c --link "<LINK_C>" --force
```

## Р Р°Р·РґРµР»РµРЅРёРµ РјР°СЂС€СЂСѓС‚РёР·Р°С†РёРё РїРѕ CIDR

```console
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client redirect add --path /etc/xp2p --config-dir config-client --cidr 10.0.102.0/24 --tag proxy-10-63-30-12
```

## Р РµР¶РёРј full-tunnel

Full-tunnel mode РґРѕСЃС‚СѓРїРµРЅ С‚РѕР»СЊРєРѕ РєРѕРіРґР° РєР»РёРµРЅС‚ СЂР°Р±РѕС‚Р°РµС‚ РІ TUN mode (`client.tun_enabled = true`).
РћРЅ Р·Р°РјРµРЅСЏРµС‚ default routes РЅР° TUN РёРЅС‚РµСЂС„РµР№СЃ, РґРѕР±Р°РІР»СЏРµС‚ bypass routes РєРѕ РІСЃРµРј РЅР°СЃС‚СЂРѕРµРЅРЅС‹Рј endpoints
Рё РїРµСЂРµРєР»СЋС‡Р°РµС‚ DNS resolvers РЅР° `client.dns_servers` РЅР° РІСЂРµРјСЏ Р°РєС‚РёРІРЅРѕСЃС‚Рё full-tunnel.

РџРµСЂРµРєР»СЋС‡РёС‚СЊ С‡РµСЂРµР· CLI:

```console
xp2p client mode tun full
```

Р’РµСЂРЅСѓС‚СЊСЃСЏ РЅР° split-tunnel:

```console
xp2p client mode tun split
```

Р’РµСЂРЅСѓС‚СЊСЃСЏ РІ proxy mode:

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

РќР° Windows Server 2016 Wintun-Р°РґР°РїС‚РµСЂ РјРѕР¶РµС‚ РїРµСЂРёРѕРґРёС‡РµСЃРєРё РѕСЃС‚Р°РІР°С‚СЊСЃСЏ disconnected РїРѕСЃР»Рµ СЂРµСЃС‚Р°СЂС‚РѕРІ (IPv4 РѕСЃС‚Р°С‘С‚СЃСЏ `Tentative`, routes РЅРµ РїСЂРёРјРµРЅСЏСЋС‚СЃСЏ). Р’ СЌС‚РѕРј СЃР»СѓС‡Р°Рµ `xp2p` РґРµСЂР¶РёС‚ СЃРјРµРЅСѓ СЂРµР¶РёРјР° РІ pending Рё РїРѕРІС‚РѕСЂСЏРµС‚ РїРѕРїС‹С‚РєРё С‡РµСЂРµР· СЂРµСЃС‚Р°СЂС‚С‹ СЃРµСЂРІРёСЃР°, РїРѕРєР° Р°РґР°РїС‚РµСЂ РЅРµ СЃС‚Р°РЅРµС‚ `up`/`preferred`. Cleanup РІС‹РїРѕР»РЅСЏРµС‚СЃСЏ РїРµСЂРµРґ РєР°Р¶РґС‹Рј СЃС‚Р°СЂС‚РѕРј. РЎР»РµРґСѓСЋС‰РёРµ СЃРѕРѕР±С‰РµРЅРёСЏ xray РѕР¶РёРґР°РµРјС‹ РїСЂРё РїРµСЂРµСЃРѕР·РґР°РЅРёРё Р°РґР°РїС‚РµСЂР°: `Failed to find matching adapter name`, `Removed orphaned adapter`.

#### РљРѕРЅС‚СЂР°РєС‚ СЃС‚Р°Р±РёР»СЊРЅРѕСЃС‚Рё full-tunnel

Full-tunnel вЂ” СЌС‚Рѕ runtime mode СЃРµСЂРІРёСЃР° Рё РѕРЅ РґРѕР»Р¶РµРЅ РѕСЃС‚Р°РІР°С‚СЊСЃСЏ Р°РєС‚РёРІРЅС‹Рј, РїРѕРєР° Desired mode вЂ” full-tunnel.

- Р РµСЃС‚Р°СЂС‚С‹ СЃРµСЂРІРёСЃР° РёР·-Р·Р° apply/watchers РЅРµ РґРѕР»Р¶РЅС‹ РѕС‚РєР°С‚С‹РІР°С‚СЊ routes РёР»Рё DNS, РµСЃР»Рё Desired РѕСЃС‚Р°С‘С‚СЃСЏ `tun_mode=full`.
- РљРѕРіРґР° Р°РґР°РїС‚РµСЂ РЅРµ РіРѕС‚РѕРІ (`Tentative` / disconnected / РѕС‚СЃСѓС‚СЃС‚РІСѓРµС‚ IPv4), runtime РґРµСЂР¶РёС‚ full-tunnel РІ pending Рё РїРѕРІС‚РѕСЂСЏРµС‚ bring-up Р°РґР°РїС‚РµСЂР° РјРµР¶РґСѓ СЂРµСЃС‚Р°СЂС‚Р°РјРё (СЃ rate limits).
- Routes Рё DNS override РґРѕР»Р¶РЅС‹ РїСЂРёРјРµРЅСЏС‚СЊСЃСЏ С‚РѕР»СЊРєРѕ РїРѕСЃР»Рµ С‚РѕРіРѕ, РєР°Рє Р°РґР°РїС‚РµСЂ СЃРѕРѕР±С‰Р°РµС‚ `up`/`preferred`, С‡С‚РѕР±С‹ РёР·Р±РµР¶Р°С‚СЊ connectivity flapping.

##### Р—Р°РґРµСЂР¶РєР° РїРѕРІС‚РѕСЂРѕРІ РІ pending

РљРѕРіРґР° full-tunnel СЏРІР»СЏРµС‚СЃСЏ Desired, РЅРѕ Р°РґР°РїС‚РµСЂ РЅРµ РіРѕС‚РѕРІ, runtime РІС…РѕРґРёС‚ РІ `FullPending` Рё Р»РѕРіРёСЂСѓРµС‚:

- `full-tunnel pending; deferring route apply until restart`

РџРѕРІС‚РѕСЂС‹ РёСЃРїРѕР»СЊР·СѓСЋС‚ exponential backoff СЃ РјР°РєСЃРёРјСѓРјРѕРј 30 СЃРµРєСѓРЅРґ (РЅР°С‡РёРЅР°СЏ СЃ 2 СЃРµРєСѓРЅРґ). Pending state Рё СЂР°СЃРїРёСЃР°РЅРёРµ РїРѕРІС‚РѕСЂРѕРІ СЃРѕС…СЂР°РЅСЏСЋС‚СЃСЏ РІ `CONFIG_ROOT/xp2p-client.tun-full.json` (`phase`, `pending_reason`, `retry_count`, `next_retry_at`), С‡С‚РѕР±С‹ СЂРµСЃС‚Р°СЂС‚С‹ СЃР»РµРґРѕРІР°Р»Рё РѕРґРЅРѕРјСѓ РєРѕРЅС‚СЂР°РєС‚Сѓ.

## DNS РјР°СЂС€СЂСѓС‚РёР·Р°С†РёСЏ РїРѕ РґРѕРјРµРЅР°Рј (С‚РѕР»СЊРєРѕ Linux/OpenWrt)

```console
xp2p client dns-forward add -d corp.test.com -t 10.0.101.142:53 --with-forward
xp2p client dns-forward add -d lab.test.com -t 10.0.102.142:53 --with-forward
```

## РћС‡РёСЃС‚РєР°

```console
xp2p client redirect remove --path /etc/xp2p --config-dir config-client --cidr 10.0.101.0/24 --tag proxy-10-63-30-11
xp2p client dns-forward remove -d corp.test.com --with-forward
xp2p client remove --path /etc/xp2p --config-dir config-client --all --ignore-missing --quiet
```
