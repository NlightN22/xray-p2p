# xray-p2p Repository Instructions

This repository delivers a minimal Trojan tunnel based on **xray-core**.

## Repository identity

- Project name: `xray-p2p`

## Language rules

- Write all repository artifacts in English.
- All code and comments inside the code must be written in English.

## Documentation rules

- For normal installation and day-to-day usage, documentation must show commands with default parameters only.
- Omit optional flags unless they are strictly required for the described flow.
- Put optional flags into clearly labeled “Advanced” or “Troubleshooting” sections and explain why they are needed.
- When documenting services/packages, do not show service invocations that rely on CLI flags; document changes via Desired inputs instead.

## Preferred technologies

- Main code languages: `sh`, `go`

## Coding style

- Keep the coding style minimalistic, concise, and without excess.
- Keep comments minimal and add them only when they are necessary for understanding.

## Architecture and project maintenance

- This project is released, so breaking changes are not allowed by default.
- Preserve backward compatibility for CLI, configs, file layouts, and service behavior unless a breaking release is explicitly planned.
- When a breaking change is unavoidable, include a documented migration plan (and ideally an automated config migration path) and follow semantic versioning.
- When renaming files or folders, check all dependencies between them and keep compatibility shims when feasible.
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
- CLI commands must only update Desired inputs under `CONFIG_ROOT` and apply marker files under `.state/`.
- OS-level changes such as TUN setup, routes, and `nftables` must be applied only by `xp2p run` or the service layer.
- Desired inputs are user-owned and must not be rewritten by the runtime/service layer.
- Apply flow is strict: Desired inputs are the source of truth. Apply compiles Desired inputs into Live runtime artifacts atomically and may keep an LKG snapshot for rollback.
- Runtime behavior (service run, diagnostics, ping, OS routing) reads Live runtime artifacts only and never reads Desired inputs directly.
- Allowed exceptions to the live-only runtime rule:
  - Deploy validation may start temporary xray-core using a compiled config derived from Desired inputs without touching Live.

## Testing rules

- Tests must read Desired inputs only when asserting staged configuration changes.
- Tests must read live config when asserting runtime behavior or service state.
- Failure dumps should include the full `.state` tree structure (live/lkg/apply.request/apply.error) when available.
- When asked to "run tests" without a specific file/test, run the full suite for the requested platform directory as a single pytest run (`tests/host/win`, `tests/host/linux`, or `tests/host/openwrt`), not one-by-one files.
- Always run pytest with `-vv -s` and tee output into `.logs/tests/` using a timestamped filename.
- Example (Linux): `pytest tests\\host\\linux -vv -s 2>&1 | Tee-Object -FilePath (".\\.logs\\tests\\pytest-linux-all-{0}.log" -f (Get-Date -Format "yyyyMMdd-HHmmss"))`
- Example (Windows): `pytest tests\\host\\win -vv -s 2>&1 | Tee-Object -FilePath (".\\.logs\\tests\\pytest-win-all-{0}.log" -f (Get-Date -Format "yyyyMMdd-HHmmss"))`
- Do not create new aggregator modules like `tests/host/_env.py` (or grow `tests/host/**/env.py` into a dumping ground).
- `tests/host/**/env.py` is allowed only as a thin compatibility facade: constants + minimal module-level state (only if required for existing behavior) + re-export of public API from focused `tests/host/**/_*.py` modules.
- Hard limit: 200 lines per module; if exceeded, split by responsibility into `tests/host/**/_*.py`.
- Do not change test imports, public names, or function signatures; keep API stable via re-export.
- Helper modules may import `from . import env as _env` only to access facade-held state; otherwise avoid env-based dependency chains.

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
- All log messages and error strings must not include any `xp2p`-style prefixes (for example `xp2p:` or `[xp2p]`); prefixes/formatting belong to the logger/CLI formatter only.

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
