# Configuration

This page describes configuration inputs and where xp2p looks for them. For the strict Desired -> Live apply rules, see [Apply flow](../flows/apply-flow.md) and [Config compilation](../flows/config-compilation.md).

## Config root

By default, xp2p uses `XP2P_CONFIG_ROOT` when set, otherwise platform defaults (for example `/etc/xp2p` on Linux/OpenWrt).

## Load order (CLI)

When a command loads configuration, it merges settings in this order:

1. Built-in defaults
2. Optional config file(s)
3. Environment variables
4. CLI overrides

By default, it loads `xp2p-client.toml` and `xp2p-server.toml` from the config root; override with `--config path/to/file`. TOML and YAML are supported.

Settings map 1:1 to environment variables via the `XP2P_` prefix (`XP2P_SERVER_INSTALL_DIR`, `XP2P_CLIENT_SERVER_ADDRESS`, etc.). A sample file lives at `config_templates/xp2p.example.yaml`.

## Editor schema

The repository includes Taplo-compatible JSON schemas for TOML Desired inputs:

- `schemas/xp2p-client.schema.json`
- `schemas/xp2p-server.schema.json`

VS Code with the Taplo extension reads `taplo.toml` from the repository root and applies these schemas to `xp2p-client.toml` and `xp2p-server.toml`. The schemas cover xp2p TOML inputs only; they do not validate generated Xray JSON artifacts.

## Global flags

Every command shares global flags such as `--config`, `--log-level` (`debug|info|warn|error`), `--log-json`, and `--version`.

Advanced / troubleshooting:

- Override the config file path with `--config path/to/file` for one-off runs.
- On Windows, `xp2p client|server service start --log-level <level>` can persist `XP2P_LOG_LEVEL` into the service environment for worker processes. Packages and services still run with default parameters.

## Xray version check

Runtime checks validate the pinned xray version before launch. Override with:

- `XP2P_XRAY_SKIP_VERSION_CHECK=1` (skip the check)
- `XP2P_XRAY_ALLOW_MISMATCH=1` (warn and continue on mismatches)

## Xray asset files

Use `xray_assets` when routing rules reference xray-core `.dat` assets such as `geoip.dat`, `geosite.dat`, or `ext:<file>:...` files.

```toml
[[xray_assets.files]]
name = "geoip.dat"
url = "https://example.com/geoip.dat"

[[xray_assets.files]]
name = "geosite.dat"
url = "https://example.com/geosite.dat"
```

During service/run startup, xp2p checks the Live `xray.json` and the configured asset list before starting xray-core. Missing configured files are downloaded into the xray asset directory. Missing files found in routing rules but not configured fail startup with a clear preflight error.

Advanced:

- `xray_assets.stale_after` sets a shared refresh interval for all configured files.
- `xray_assets.files[].stale_after` overrides the shared refresh interval for one file.
- Empty or `0` `stale_after` disables periodic refresh while still requiring the file to exist.
- xp2p uses `XRAY_LOCATION_ASSET` when it is set; otherwise it uses the directory that contains the resolved `xray` binary.
