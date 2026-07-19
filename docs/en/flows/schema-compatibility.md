# Schema compatibility

Schema drift and backward compatibility are separate release gates. Drift proves that generated JSON Schema matches current Go models. Compatibility proves that Desired inputs accepted by the previous release remain usable after upgrade.

For each release:

1. Use the previous release tag or its merge base as the source of persisted contracts.
2. Add representative client and server Desired files under `tests/schema/compat/<version>/`.
3. Run `make schema-compat`. Each fixture must pass the current JSON Schema and current owner-package runtime decoder/normalizer.
4. Review removed, renamed, newly required, type-changed, enum-restricted, and otherwise tightened fields.
5. Follow the [Normalization pipeline](normalization-pipeline.md) for legacy syntax. Keep legacy fields explicit in strict schemas until their documented removal version.
6. Record required user action in release upgrade notes. Do not treat generated-schema equality as compatibility evidence.

## v0.2.7 audit

The `v0.2.6` release did not contain JSON Schema files, so the baseline is the TOML and persisted state accepted by its runtime code. Versioned fixtures cover base client/server settings, client endpoints, redirects, reverse channels, forwards, legacy server Trojan users, server redirects, server reverse channels, server forward rules, and Xray log overrides.

The audit found and fixed a schema key mismatch: server forwards are persisted under `server.forward_rules`, not `server.forwards`. New v0.2.7 fields are additive or optional. Legacy `server.trojan_users` remains explicitly accepted and is normalized by the server owner package. The compatibility fixtures pass both current schemas and current runtime decoders.
