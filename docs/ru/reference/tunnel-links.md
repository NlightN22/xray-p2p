# Tunnel Links

Этот документ определяет модель tunnel link в xp2p. Это стандарт проектирования
для будущего кода и тестов.

xp2p использует стандартные protocol share links как основной формат обмена:
Trojan tunnels используют `trojan://`, а VLESS tunnels используют `vless://`.
Метаданные, специфичные для xp2p, кодируются как опциональные namespaced query
parameters в этих ссылках.

## Goals

- Позволить пользователям соединять два endpoint-а без изучения деталей Xray
  transport.
- Оставить generated links импортируемыми во внешние Xray-compatible clients.
- Сохранить поддержку self-signed TLS certificates.
- Запретить plaintext tunnel profiles.
- Сохранить работу legacy Trojan links.

## Non-Goals

- Не вводить публичные `xp2p://` links для обычных tunnel connections.
- Не показывать выбор transport protocol в обычных install workflows.
- Не генерировать tunnel links с `security=none`.

`xp2p://` может быть введён позже только для provisioning bundle, который не
является стандартной connection link и не ожидается к импорту во внешние clients.

## Terms

- **Share link**: protocol link, например `trojan://...` или `vless://...`.
- **Profile**: управляемый xp2p recipe, который отображается в Xray protocol,
  transport, security, flow и credentials.
- **Extension parameter**: query parameter, принадлежащий xp2p и начинающийся с
  `xp2p_`.
- **Endpoint identity**: внутренние нормализованные данные для deduplication и
  state updates. Они не должны заменять standard share links.

## Principles

1. Standard share links являются источником истины для connection data.
2. URL scheme должен соответствовать выбранному protocol.
3. Unknown query parameters не должны заставлять xp2p отклонять ссылку.
4. Unknown parameters следует сохранять, когда xp2p переписывает ссылку.
5. Параметры, принадлежащие xp2p, должны использовать префикс `xp2p_`.
6. Пользователь должен видеть одну generated link и одно install action.
7. Internal normalization разрешена только для comparison или migration.

## Internal Field Mapping

Общая tunnel model использует protocol-neutral fields. Protocol codecs
переводят эти fields в конкретные share-link parameters и обратно.

| Internal field | Trojan link | VLESS link |
| --- | --- | --- |
| `protocol` | `trojan` scheme | `vless` scheme |
| `credential` | password in `trojan://PASSWORD@...` | UUID/id in `vless://UUID@...` |
| `host` | URL host | URL host |
| `port` | URL port | URL port |
| `security` | `security=tls` | `security=tls` or `security=reality` |
| `server_name` | `sni` | `sni` |
| `user_label` | URL fragment | URL fragment |
| `transport` | implicit `tcp`, or explicit `type=tcp` if rendered | `type=tcp` or `type=xhttp` |
| `flow` | not used | `flow=xtls-rprx-vision` when the profile requires Vision |
| `protocol_encryption` | not used | `encryption=none` unless the selected Xray profile requires another value |
| `tls_pin_sha256` | `pinnedPeerCertSha256` or `xp2p_pin_sha256` | `xp2p_pin_sha256` unless a target-compatible VLESS key is selected |
| `tls_verify_name` | `verifyPeerCertByName` or `xp2p_verify_name` | `xp2p_verify_name` unless a target-compatible VLESS key is selected |
| `reality_public_key` | not used | `pbk` |
| `reality_short_id` | not used | `sid` |
| `fingerprint` | optional TLS client fingerprint when supported | `fp` |
| `xhttp_path` | not used | `path` |
| `xhttp_host` | not used | `host` query parameter |
| `xhttp_mode` | not used | `mode` |

`credential` намеренно generic. Для Trojan это password; для VLESS это user
UUID/id. Server-side user label является отдельным field и отображается в link
fragment или protocol metadata, например VLESS `email`.

## Security Baseline

Все tunnel profiles, управляемые xp2p, должны быть encrypted. Generator и
parser должны отклонять managed profiles, которые сводятся к plaintext transport
security.

Разрешённые значения transport security:

- `tls`
- `reality`

Запрещённые значения для generated managed profiles:

- `none`
- пустой security, когда он означает plaintext

Legacy imports могут defensively парсить существующие links, но xp2p не должен
генерировать plaintext tunnel links.

## Sources

Форматы ниже следуют de-facto conventions для Xray client share-link. Они не
считаются RFC-stable contract, поэтому parsers должны быть permissive, а
renderers conservative.

- Xray VLESS inbound/outbound configuration:
  <https://xtls.github.io/en/config/inbounds/vless.html> and
  <https://xtls.github.io/en/config/outbounds/vless.html>
- Xray TLS transport settings:
  <https://xtls.github.io/en/config/transports/tls.html>
- Xray REALITY transport settings:
  <https://xtls.github.io/en/config/transports/reality.html>
- XHTTP design notes:
  <https://github.com/XTLS/Xray-core/discussions/4113>

## Profiles

Profile names являются internal. Обычно пользователи должны запрашивать `auto`,
а xp2p выбирает profile из server capabilities.

| Profile | Scheme | Protocol | Transport | Security | Flow | Self-signed TLS |
| --- | --- | --- | --- | --- | --- | --- |
| `vless-tls-vision` | `vless://` | `vless` | `tcp` / raw | `tls` | `xtls-rprx-vision` | Supported through certificate pinning |
| `vless-reality-vision` | `vless://` | `vless` | `tcp` / raw | `reality` | `xtls-rprx-vision` | Not applicable |
| `vless-xhttp-tls` | `vless://` | `vless` | `xhttp` | `tls` | none unless required by current Xray | Supported through certificate pinning |
| `vless-xhttp-reality` | `vless://` | `vless` | `xhttp` | `reality` | none unless required by current Xray | Not applicable |
| `trojan-tls-legacy` | `trojan://` | `trojan` | `tcp` / raw | `tls` | none | Supported through certificate pinning |

Default selection:

1. Предпочитать `vless-tls-vision`, когда доступен TLS certificate material.
2. Использовать `vless-reality-vision`, когда server настроен на REALITY вместо
   обычного TLS certificate material.
3. Использовать XHTTP profiles только когда deployment требует XHTTP transport
   behavior.
4. Использовать `trojan-tls-legacy` для существующих Trojan deployments и
   импортированных `trojan://` links.

## VLESS Parameters

Base shape:

```text
vless://UUID@host:port?key=value#remark
```

| Parameter | Meaning |
| --- | --- |
| userinfo UUID | VLESS user id |
| host | Endpoint host or IP |
| port | Endpoint port |
| `encryption` | Render as `none` unless the selected Xray profile requires another value. |
| `type` | Transport type, for example `tcp` or `xhttp`. |
| `security` | `tls` or `reality`. |
| `flow` | VLESS flow, for example `xtls-rprx-vision`. |
| `sni` | TLS SNI or REALITY server name. |
| `alpn` | Comma-separated ALPN values when required. |
| `fp` | Fingerprint value, for example `chrome`. |
| `pbk` | REALITY public key. |
| `sid` | REALITY short id. |
| `spx` | REALITY spiderX value. |
| `path` | HTTP/XHTTP path when transport requires it. |
| `host` | HTTP host header when transport requires it. |
| `mode` | XHTTP mode when supported by the target client. |

### VLESS + TLS + Vision

```text
vless://UUID@host:port?encryption=none&type=tcp&security=tls&flow=xtls-rprx-vision&sni=server.example#remark
```

xp2p requires UUID, host, port, `type=tcp`, `security=tls` и
`flow=xtls-rprx-vision`. `sni` required, когда используется certificate name
validation или pin-by-name.

Self-signed TLS поддерживается. xp2p should prefer certificate pinning over
`allowInsecure`.

### VLESS + REALITY + Vision

```text
vless://UUID@host:port?encryption=none&type=tcp&security=reality&flow=xtls-rprx-vision&sni=server.example&fp=chrome&pbk=PUBLIC_KEY&sid=SHORT_ID&spx=%2F#remark
```

xp2p requires UUID, host, port, `type=tcp`, `security=reality`,
`flow=xtls-rprx-vision`, `sni`, `fp`, `pbk` и `sid`.

REALITY не использует `cert.pem` и `key.pem` как обычный TLS server certificate
material. Поддержка self-signed TLS certificates не применяется к этому profile.

### VLESS + XHTTP + TLS

```text
vless://UUID@host:port?encryption=none&type=xhttp&security=tls&sni=server.example&path=%2Fxp2p%2Fgenerated#remark
```

xp2p requires UUID, host, port, `type=xhttp`, `security=tls`, generated `path`
и `sni`, когда используется certificate name validation или pin-by-name.
Optional fields включают `mode`, `host` и `alpn`.

Self-signed TLS поддерживается через ту же pinning model, что и
`vless-tls-vision`.

### VLESS + XHTTP + REALITY

```text
vless://UUID@host:port?encryption=none&type=xhttp&security=reality&sni=server.example&fp=chrome&pbk=PUBLIC_KEY&sid=SHORT_ID&spx=%2F&path=%2Fxp2p%2Fgenerated#remark
```

xp2p requires UUID, host, port, `type=xhttp`, `security=reality`, generated
`path`, `sni`, `fp`, `pbk` и `sid`.

Считай этот profile advanced, пока XHTTP behavior не покрыт smoke tests against
the pinned Xray version.

## Trojan Parameters

Base shape:

```text
trojan://PASSWORD@host:port?security=tls&sni=server.example#remark
```

| Parameter | Meaning |
| --- | --- |
| userinfo password | Trojan password |
| host | Endpoint host or IP |
| port | Endpoint port |
| `security` | Must be `tls` for xp2p-managed links. |
| `sni` | TLS SNI / server name. |
| `alpn` | Comma-separated ALPN values when required. |
| `allowInsecure` | Disable TLS verification. Avoid in generated default links. |

Self-signed TLS поддерживается. Существующие parameters
`pinnedPeerCertSha256` и `verifyPeerCertByName` остаются поддерживаемыми xp2p
для current deployments.

## xp2p Extensions

Extension parameters должны быть optional для external clients. External
clients, которые их не понимают, всё равно должны импортировать tunnel через
standard fields.

Reserved prefix:

```text
xp2p_
```

| Parameter | Meaning |
| --- | --- |
| `xp2p_v` | Extension schema version. |
| `xp2p_profile` | Internal profile id used by xp2p. |
| `xp2p_exp` | Optional Unix expiration timestamp for deploy-style links. |
| `xp2p_user` | Optional stable xp2p user id when the remark is insufficient. |
| `xp2p_pin_sha256` | Certificate pin when no broadly compatible standard key is available. |
| `xp2p_verify_name` | Certificate name to verify together with a pin. |

Rules:

- Игнорировать unknown non-`xp2p_` parameters, если они не конфликтуют с
  profile.
- Отклонять unknown `xp2p_` parameters только когда `xp2p_v` требует strict
  parsing.
- Не помещать новые secrets в extensions, если те же данные уже не являются
  частью standard share link.
- Extensions не должны менять meaning standard fields для external clients.

## Parser Rules

1. Parse the URL scheme.
2. Dispatch to the protocol codec (`trojan` or `vless`).
3. Parse standard fields into a tunnel endpoint.
4. Parse known `xp2p_*` extensions.
5. Preserve unknown query parameters for round-trip output.
6. Infer a profile only when fields match exactly one supported profile.
7. Reject generated/apply paths that would produce plaintext transport
   security.

Unknown parameters are compatibility data, not errors.

## Renderer Rules

1. Render standard share-link fields first.
2. Render only parameters required for the selected profile plus stable
   compatibility fields.
3. Add `xp2p_*` parameters only when xp2p needs them for state, deployment, or
   migration.
4. Keep parameter names stable once released.
5. Do not render plaintext profiles.

Renderer modes:

- `standard`: fields expected by common external clients.
- `xp2p-extended`: standard fields plus `xp2p_*` metadata.

Используй `xp2p-extended` только когда extension metadata требуется для
one-click xp2p workflow.

## State and Compatibility

Desired endpoint state должен хранить structured data, а не только raw link. Он
также должен сохранять достаточно raw query data, чтобы заново render compatible
link.

Required endpoint state:

- profile id
- protocol
- host
- port
- credentials
- transport type
- security type
- TLS material or REALITY material
- extension parameters
- unknown preserved parameters

Backward compatibility:

- Existing Trojan endpoint state без profile metadata означает
  `trojan-tls-legacy`.
- Existing `trojan://` deploy links остаются valid.
- New VLESS profiles не должны ломать existing Trojan CLI flows.

## Validation Matrix

| Condition | Result |
| --- | --- |
| `security=none` in a generated managed link | Reject |
| Missing VLESS UUID | Reject |
| Missing Trojan password | Reject |
| VLESS Vision without `flow=xtls-rprx-vision` | Reject for Vision profile |
| REALITY without `pbk` or `sid` | Reject |
| TLS profile without trusted CA, pin, or explicit insecure mode | Reject for default workflows |
| Unknown non-`xp2p_` query parameter | Preserve and ignore |
| Unknown strict-version `xp2p_` parameter | Reject only if schema version requires it |

## Implementation Boundaries

Link layer должен возвращать structured tunnel endpoint data. Он не должен
строить полный Xray JSON.

Suggested package boundaries:

```text
go/internal/tunnel/link
  ParseShareLink
  RenderShareLink
  TrojanCodec
  VLESSCodec
  ExtensionCodec

go/internal/tunnel/profile
  ResolveAutoProfile
  ValidateProfile
  BuildClientOutbound
  BuildServerInbound
```

Client и server layers должны зависеть от structured tunnel endpoints и
profiles. Они не должны hand-roll protocol-specific URL query logic.
