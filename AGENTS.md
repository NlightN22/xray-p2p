# xray-p2p Repository Instructions

This repository delivers a minimal Trojan tunnel based on **xray-core**.

## Repository identity

- Project name: `xray-p2p`

## Language rules

- Write all repository artifacts in English.
- All code and comments inside the code must be written in English.

## Preferred technologies

- Main code languages: `sh`, `go`

## Coding style

- Keep the coding style minimalistic, concise, and without excess.
- Keep comments minimal and add them only when they are necessary for understanding.

## Architecture and project maintenance

- This is a new project, so file and folder names can be changed without backward compatibility constraints.
- When renaming files or folders, check all dependencies between them.
- Try to use Python for editing and viewing when it is available.

## File size and decomposition

- Try to keep file size under 300 lines.
- If a file becomes longer, prefer decomposition.
- A slightly longer file can be tolerated when there is a good reason, but large files should be split.

## Line endings

- All new files must use LF line endings.

## Shell scripting rules

- All Linux shell scripts must be compatible with OpenWrt and the `ash` shell.
- For testing shell scripts, use Vagrant when needed.
- The test environment is located in `infra/vagrant`.
- The Vagrant environment scheme for shell scripts is located in `infra/vagrant/openwrt/scheme.drawio`.
- Packages must create required directories during installation. Build scripts must not add directory creation as a workaround for packaging omissions.

## Go rules

- Use a common logger for all output.

## Services and runtime behavior

- System services (`systemd`, `procd`, `Windows SCM`) must not rely on CLI flags.
- Packages must run `xp2p` binaries with default parameters only.
- CLI commands must only update configuration and state files.
- OS-level changes such as TUN setup, routes, and `nftables` must be applied only by `xp2p run` or the service layer.

## CLI standards

- Identical long flag names must use the same short aliases everywhere.
- Verify flag consistency when adding new commands.
- Long flags must always have short aliases.
- Short aliases must be unique for each parameter across the project.
- See `commands_map` when checking or introducing flag aliases.
- When adding flags, include them in `ValidArgs` or `ValidArgsFunction`.

## Logging rules

- Log files must live under `XP2P_LOG_ROOT`.
- The default log root is `/var/log/xp2p`.
- The audit log file name must be `audit.log`.

## UI and use case boundaries

- UI flows must call internal use cases or platform packages directly.
- Do not shell out to the `xp2p` CLI from the UI layer.
- UI must validate required inputs before executing install actions.
- UI must use configuration defaults for optional fields when they are empty.
- Keep presentation logic separate from install and service logic.
- App bindings should call use cases.
- The UI layer should only pass data.

## Terminology

- `cidr` means a network in CIDR format, for example `10.0.0.0/24`.
- Use the term `cidr` for redirect and NAT rules.
- Do not use `subnet` for this meaning.
- `host` means a DNS name or IP address of a node or server.
- `target` means a destination in `host:port` format.
- Use `target` for forwarding and `dns-forward` behavior.

## Windows-specific notes

- Windows `xray-core` logs such as `Failed to find matching adapter name` and `Removed orphaned adapter` are expected during Wintun setup.
- Do not treat these messages as errors unless the actual behavior is broken.
