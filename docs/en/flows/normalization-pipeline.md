# Normalization Pipeline

This page defines how xp2p evolves project-owned persisted data without adding schema versions to user files or maintaining version-by-version migration chains.

The normalization pipeline applies to xp2p-owned persisted inputs and state files, for example:

- TOML application configuration.
- JSON state files such as `dns-forward-state.json`.
- Future xp2p-owned metadata files.

It does not apply to Xray JSON. Xray JSON is an external runtime format and must be handled by the Xray config compiler and validators only.

## Model

Each domain package owns its raw and canonical models. Shared normalization code provides only generic primitives.

```text
Raw decode
  -> Defaults
  -> Compatibility rules
  -> Validation
  -> Canonical model
  -> Optional canonical write
```

Application code must use the canonical model. Legacy fields and old syntax must be interpreted only inside compatibility rules.

## Shared Package

The shared package lives under `go/internal/normalize/` and contains:

- `Report`: warnings, deprecated fields, applied rules, and notes.
- `Rule`: stable rule name, description, `DeprecatedSince`, `RemovedSince`, removal note, and apply function.
- `Pipeline`: defaults, compatibility rules, validation, and canonical model construction.

The shared package must not know domain fields such as `auto_forward`, `forward_owner`, or `server.address`.

## Domain Package Layout

Domain-specific persisted data should keep normalization code near the domain:

```text
go/internal/<domain>/
  <name>_raw.go
  <name>_canonical.go
  <name>_compat.go
  <name>_normalize.go
```

The raw model accepts current and legacy syntax. The canonical model contains only fields that application logic is allowed to use.

## Compatibility Rules

Compatibility is explicit and time-boxed. A rule must set:

- `Name`: stable identifier used in reports and tests.
- `Description`: what the rule does.
- `DeprecatedSince`: first xp2p version where the old syntax is deprecated.
- `RemovedSince`: first xp2p version where the old syntax must be rejected.
- `RemovalNote`: short replacement guidance.

When the current xp2p version is greater than or equal to `RemovedSince`, the rule must reject the removed syntax instead of normalizing it.

Do not add `schema_version` to user TOML, JSON state, or metadata files for this purpose. Treat old fields as legacy syntax.

## Canonical Writes

Normal reads must not rewrite files automatically. A file is written in canonical format only during a normal write path for that file.

For example, a state file loaded from legacy syntax remains unchanged until a command changes and saves that state. The saved file must omit deprecated fields and include the canonical fields.

Future explicit normalization commands can use the same pipeline with modes such as:

- `check`
- `normalize --print`
- `normalize --write`

## DNS Forward State

`dns-forward-state.json` is the first JSON state file using this approach.

Legacy syntax:

```json
{
  "auto_forward": true
}
```

Canonical syntax:

```json
{
  "forward_owner": "dns-forward"
}
```

Compatibility rule:

- If `forward_owner` is present, it is authoritative.
- If `forward_owner` is absent and `auto_forward` is `true`, normalize to `forward_owner = "dns-forward"`.
- If `auto_forward` is `false` or absent, normalize to no dns-forward ownership.
- If legacy and canonical fields conflict, return a normalization error.
- Starting with xp2p `0.2.8`, `auto_forward` is rejected.

`dns-forward remove` uses only canonical ownership:

- It removes the dnsmasq domain entry.
- It checks whether this is the last dns-forward domain using the same forward listen port.
- If this is the last domain and `forward_owner == "dns-forward"`, it removes the xray forward.
- If the forward is not owned by dns-forward, it leaves the xray forward in place.
