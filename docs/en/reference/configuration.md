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

## Global flags

Every command shares global flags such as `--config`, `--log-level` (`debug|info|warn|error`), `--log-json`, and `--version`.

Advanced / troubleshooting:

- Override the config file path with `--config path/to/file` for one-off runs.
- On Windows, `xp2p client|server service start --log-level <level>` can persist `XP2P_LOG_LEVEL` into the service environment for worker processes. Packages and services still run with default parameters.

## Xray version check

Runtime checks validate the pinned xray version before launch. Override with:

- `XP2P_XRAY_SKIP_VERSION_CHECK=1` (skip the check)
- `XP2P_XRAY_ALLOW_MISMATCH=1` (warn and continue on mismatches)
