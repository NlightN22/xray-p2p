# Tunnel Links

This document defines the xp2p tunnel link model. It is a design standard for
future code and tests.

xp2p uses standard protocol share links as the primary interchange format:
Trojan tunnels use `trojan://`, and VLESS tunnels use `vless://`. xp2p-specific
metadata is encoded as optional namespaced query parameters on those links.

## Goals

- Let users connect two endpoints without learning Xray transport details.
- Keep generated links importable by external Xray-compatible clients.
- Preserve support for self-signed TLS certificates.
- Forbid plaintext tunnel profiles.
- Keep legacy Trojan links working.

## Non-Goals

- Do not introduce public `xp2p://` links for ordinary tunnel connections.
- Do not expose transport protocol choices in normal install workflows.
- Do not generate `security=none` tunnel links.

`xp2p://` may be introduced later only for a provisioning bundle that is not a
standard connection link and is not expected to import into external clients.

## Terms

- **Share link**: A protocol link such as `trojan://...` or `vless://...`.
- **Profile**: An xp2p-managed recipe that maps to Xray protocol, transport,
  security, flow, and credentials.
- **Extension parameter**: An xp2p-owned query parameter prefixed with
  `xp2p_`.
- **Endpoint identity**: Internal normalized data used for deduplication and
  state updates. It must not replace standard share links.

## Principles

1. Standard share links are the source of truth for connection data.
2. The URL scheme must match the selected protocol.
3. Unknown query parameters must not make xp2p reject a link.
4. Unknown parameters should be preserved when xp2p rewrites a link.
5. xp2p-owned parameters must use the `xp2p_` prefix.
6. Users should see one generated link and one install action.
7. Internal normalization is allowed for comparison or migration only.

## Internal Field Mapping

The shared tunnel model uses protocol-neutral fields. Protocol codecs translate
those fields to and from concrete share-link parameters.

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

`credential` is intentionally generic. For Trojan it is the password; for VLESS
it is the user UUID/id. The server-side user label is a separate field and maps
to the link fragment or protocol metadata such as VLESS `email`.

## Security Baseline

All xp2p-managed tunnel profiles must be encrypted. The generator and parser
must reject managed profiles that resolve to plaintext transport security.

Allowed transport security values:

- `tls`
- `reality`

Forbidden values for generated managed profiles:

- `none`
- empty security when it means plaintext

Legacy imports may parse existing links defensively, but xp2p must not generate
plaintext tunnel links.

## Sources

The formats below follow de-facto Xray client share-link conventions. They are
not treated as an RFC-stable contract, so parsers must be permissive and
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

Profile names are internal. Users should normally request `auto`, and xp2p
selects a profile from server capabilities.

| Profile | Scheme | Protocol | Transport | Security | Flow | Self-signed TLS |
| --- | --- | --- | --- | --- | --- | --- |
| `vless-tls-vision` | `vless://` | `vless` | `tcp` / raw | `tls` | `xtls-rprx-vision` | Supported through certificate pinning |
| `vless-reality-vision` | `vless://` | `vless` | `tcp` / raw | `reality` | `xtls-rprx-vision` | Not applicable |
| `vless-xhttp-tls` | `vless://` | `vless` | `xhttp` | `tls` | none unless required by current Xray | Supported through certificate pinning |
| `vless-xhttp-reality` | `vless://` | `vless` | `xhttp` | `reality` | none unless required by current Xray | Not applicable |
| `trojan-tls-legacy` | `trojan://` | `trojan` | `tcp` / raw | `tls` | none | Supported through certificate pinning |

Default selection:

1. Prefer `vless-tls-vision` when TLS certificate material is available.
2. Use `vless-reality-vision` when the server is configured for REALITY instead
   of ordinary TLS certificate material.
3. Use XHTTP profiles only when the deployment needs XHTTP transport behavior.
4. Use `trojan-tls-legacy` for existing Trojan deployments and imported
   `trojan://` links.

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

xp2p requires UUID, host, port, `type=tcp`, `security=tls`, and
`flow=xtls-rprx-vision`. `sni` is required when certificate name validation or
pin-by-name is used.

Self-signed TLS is supported. xp2p should prefer certificate pinning over
`allowInsecure`.

### VLESS + REALITY + Vision

```text
vless://UUID@host:port?encryption=none&type=tcp&security=reality&flow=xtls-rprx-vision&sni=server.example&fp=chrome&pbk=PUBLIC_KEY&sid=SHORT_ID&spx=%2F#remark
```

xp2p requires UUID, host, port, `type=tcp`, `security=reality`,
`flow=xtls-rprx-vision`, `sni`, `fp`, `pbk`, and `sid`.

REALITY does not use `cert.pem` and `key.pem` as ordinary TLS server
certificate material. Self-signed TLS certificate support does not apply to this
profile.

### VLESS + XHTTP + TLS

```text
vless://UUID@host:port?encryption=none&type=xhttp&security=tls&sni=server.example&path=%2Fxp2p%2Fgenerated#remark
```

xp2p requires UUID, host, port, `type=xhttp`, `security=tls`, a generated
`path`, and `sni` when certificate name validation or pin-by-name is used.
Optional fields include `mode`, `host`, and `alpn`.

Self-signed TLS is supported through the same pinning model as
`vless-tls-vision`.

### VLESS + XHTTP + REALITY

```text
vless://UUID@host:port?encryption=none&type=xhttp&security=reality&sni=server.example&fp=chrome&pbk=PUBLIC_KEY&sid=SHORT_ID&spx=%2F&path=%2Fxp2p%2Fgenerated#remark
```

xp2p requires UUID, host, port, `type=xhttp`, `security=reality`, generated
`path`, `sni`, `fp`, `pbk`, and `sid`.

Treat this profile as advanced until XHTTP behavior is covered by smoke tests
against the pinned Xray version.

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

Self-signed TLS is supported. Existing `pinnedPeerCertSha256` and
`verifyPeerCertByName` parameters remain supported by xp2p for current
deployments.

## xp2p Extensions

Extension parameters must be optional for external clients. External clients
that do not understand them should still import the tunnel using standard
fields.

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

- Ignore unknown non-`xp2p_` parameters unless they conflict with the profile.
- Reject unknown `xp2p_` parameters only when `xp2p_v` requires strict parsing.
- Do not put new secrets into extensions unless the same data is already part
  of the standard share link.
- Extensions must not change standard field meaning for external clients.

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

Use `xp2p-extended` only when extension metadata is required for a one-click
xp2p workflow.

## State and Compatibility

Desired endpoint state must store structured data, not only the raw link. It
should also preserve enough raw query data to re-render a compatible link.

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

- Existing Trojan endpoint state without profile metadata means
  `trojan-tls-legacy`.
- Existing `trojan://` deploy links remain valid.
- New VLESS profiles must not break existing Trojan CLI flows.

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

The link layer should return structured tunnel endpoint data. It should not
build full Xray JSON.

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

Client and server layers should depend on structured tunnel endpoints and
profiles. They should not hand-roll protocol-specific URL query logic.
