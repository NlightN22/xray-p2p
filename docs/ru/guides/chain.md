# Р¦РµРїРѕС‡РєР° (C2вЂ“BвЂ“AвЂ“C1)

Р­С‚Р° С†РµРїРѕС‡РєР° РѕС‚РїСЂР°РІР»СЏРµС‚ С‚СЂР°С„РёРє РёР· C2 С‡РµСЂРµР· B Рё A, С‡С‚РѕР±С‹ РґРѕСЃС‚РёС‡СЊ C1.

## РЎС…РµРјР°

```mermaid
flowchart TB
  C1["C1 (РіРѕСЃС‚СЊ)<br/>10.0.101.0/24 Р·Р° NAT РЅР° A"] -->|"default gw"| A["A (server router)<br/>xp2ps"]
  A <-->|"xp2p TUN РїРѕРІРµСЂС… Xray"| B["B (client router)<br/>xp2pc"]
  B -->|"default gw"| C2["C2 (РіРѕСЃС‚СЊ)<br/>10.0.102.0/24 Р·Р° NAT РЅР° B"]
```

## РџСЂРµРґРїРѕСЃС‹Р»РєРё

- A = СЂРѕСѓС‚РµСЂ СЃ `xp2p server`, B = СЂРѕСѓС‚РµСЂ СЃ `xp2p client`.
- C1 РЅР°С…РѕРґРёС‚СЃСЏ Р·Р° A (`10.0.101.0/24`), C2 РЅР°С…РѕРґРёС‚СЃСЏ Р·Р° B (`10.0.102.0/24`).
- РўСѓРЅРЅРµР»СЊ AвЂ“B СѓР¶Рµ СЂР°Р±РѕС‚Р°РµС‚, Рё РѕР±Рµ СЃС‚РѕСЂРѕРЅС‹ Р·Р°РїСѓС‰РµРЅС‹ РІ СЂРµР¶РёРјРµ TUN.
- C1 РёСЃРїРѕР»СЊР·СѓРµС‚ A РєР°Рє default gateway, Р° C2 РёСЃРїРѕР»СЊР·СѓРµС‚ B РєР°Рє default gateway.

## Р РµРґРёСЂРµРєС‚С‹ (РјР°СЂС€СЂСѓС‚С‹ СЃС‚Р°РІСЏС‚СЃСЏ РЅР° A Рё B)

РљРѕРіРґР° РІРєР»СЋС‡С‘РЅ TUN, `xp2p {client,server} redirect add --cidr ...` РєРѕРјРїРёР»РёСЂСѓРµС‚СЃСЏ РІ РјР°СЂС€СЂСѓС‚С‹ РћРЎ РЅР° СЂРѕСѓС‚РµСЂР°С… (A/B) РІРѕ РІСЂРµРјСЏ apply. Р СѓС‡РЅС‹Рµ РјР°СЂС€СЂСѓС‚С‹ РЅР° C1/C2 РґРѕР±Р°РІР»СЏС‚СЊ РЅРµ РЅСѓР¶РЅРѕ.

```console
xp2p client redirect add --cidr 10.0.101.0/24
xp2p server redirect add --cidr 10.0.102.0/24
```

РџСЂРёРјРµРЅРё РёР·РјРµРЅРµРЅРёСЏ РїРµСЂРµР·Р°РїСѓСЃРєРѕРј СЃРµСЂРІРёСЃРѕРІ С‡РµСЂРµР· service manager (РЅР°РїСЂРёРјРµСЂ `service xp2p-client restart` / `service xp2p-server restart` РЅР° OpenWrt РёР»Рё `systemctl restart xp2p-client xp2p-server` РЅР° СЃРёСЃС‚РµРјР°С… СЃ systemd).

## OpenWrt firewall

РџСЂРёРІСЏР¶Рё TUN-РёРЅС‚РµСЂС„РµР№СЃ xp2p Рє firewall zone Рё СЂР°Р·СЂРµС€Рё LAN <-> tunnel forwarding.

РќР° B (client, `xp2pc`):

```console
uci -q delete firewall.xp2ptun
uci set firewall.xp2ptun='zone'
uci set firewall.xp2ptun.name='xp2ptun'
uci set firewall.xp2ptun.network='xp2pc'
uci set firewall.xp2ptun.input='ACCEPT'
uci set firewall.xp2ptun.output='ACCEPT'
uci set firewall.xp2ptun.forward='ACCEPT'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='lan'
uci set firewall.@forwarding[-1].dest='xp2ptun'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='xp2ptun'
uci set firewall.@forwarding[-1].dest='lan'

uci commit firewall
/etc/init.d/firewall restart
```

РќР° A (server, `xp2ps`) СЃРґРµР»Р°Р№ С‚Рѕ Р¶Рµ СЃР°РјРѕРµ, РЅРѕ РїРѕСЃС‚Р°РІСЊ `firewall.xp2ptun.network='xp2ps'`.

## РџСЂРѕРІРµСЂРєР°

РќР° B (client router) РїСЂРѕРІРµСЂСЊ, С‡С‚Рѕ C1 РґРѕСЃС‚СѓРїРµРЅ С‡РµСЂРµР· С‚СѓРЅРЅРµР»СЊ СЃ РїРѕРјРѕС‰СЊСЋ `xp2p ping`. Р’С‹Р±РµСЂРё РїРѕСЂС‚, РєРѕС‚РѕСЂС‹Р№ С‚РѕС‡РЅРѕ РѕС‚РєСЂС‹С‚ РЅР° C1 (РЅР°РїСЂРёРјРµСЂ `22/tcp` РґР»СЏ SSH):

```console
xp2p ping 10.0.101.1 --tunnel --proto tcp --port 22
```
