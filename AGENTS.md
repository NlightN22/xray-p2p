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
- Generate credentials, passwords, tokens, and stable user/client identifiers through `go/internal/identity`.
- New direct random or UUID generation in production code is allowed only for explicitly ephemeral values or certificate/key material.
- When adding a new generated value type, add a domain-named function to `go/internal/identity` first.

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
- When changing Go code on a Windows workstation, run native Go tests and WSL Go tests unless the user asks for a narrower check.
- For broad Go verification, use `make test` on Windows and `make test-wsl` for the Linux build view.
- Run xray smoke tests only when the change touches xray API/runtime behavior or when explicitly requested; they require the `xray_smoke` build tag and a runnable xray binary.

## Services and runtime behavior

- System services (`systemd`, `procd`, `Windows SCM`) must not rely on CLI flags.
- Packages must run `xp2p` binaries with default parameters only.
- Xray runtime API access must use top-level `api.listen` with separate client and server loopback ports. Do not add a dedicated API `dokodemo-door` inbound or API routing rule unless an explicit migration plan changes this architecture.
- Runtime changes must be designed as on-flow changes first: check whether the pinned Xray gRPC API can apply and verify the change without restarting `xray-core`.
- When a change can be applied safely through the pinned Xray gRPC API, use runtime apply instead of restarting `xray-core`.
- Do not add a restart-based path for Xray resources until the API capability has been checked and the limitation is documented.
- Runtime-capable CLI commands should build and validate a candidate config before touching Desired inputs.
- If the target service is running, runtime-capable CLI commands must apply the candidate to running Xray through gRPC, verify the runtime result, publish matching Live artifacts, and only then persist the corresponding Desired inputs under `CONFIG_ROOT`.
- If the target service is stopped or no running Live runtime is available, runtime-capable CLI commands should persist the Desired inputs only. They must not publish Live artifacts or create `apply.request` as a fake successful runtime apply; the next run/service start is responsible for compiling Desired into Live.
- If the target service appears to be running but API apply or verification fails, the command must return an error and must not change Desired or Live.
- Runtime-capable CLI commands must not use a hidden restart fallback. Their result must be explicit: applied to running Xray, staged for next start, or failed.
- Runtime apply is not complete until the successful running Xray state is persisted into Desired inputs and matching Live artifacts.
- If runtime apply succeeds but Desired/Live persistence fails, roll back the runtime change or write an explicit `apply.error`; never leave a silent drift between running Xray, Desired inputs, and Live artifacts.
- Restart/service apply is allowed only for unsupported, API-unavailable, service-layer-required, or staged-while-stopped operations. API-capable operations that fail API apply or verification while the service is running must be reported as failed runtime apply, not masked as successful deferred restart work.
- Manual Desired edits remain a separate watcher/service apply flow. They do not provide immediate CLI runtime feedback and may be diagnosed through `.state` markers, status commands, and logs.
- OS-level changes such as TUN setup, routes, DNS, firewall, and `nftables` must still be applied only by `xp2p run` or the service layer.
- Do not restart xray-core for changes that can be applied and verified through gRPC.
- Desired inputs are user-owned and must not be rewritten by the runtime/service layer.
- Apply flow is strict: after commit, Desired inputs are the source of truth. Runtime-capable operations may work from an uncommitted candidate first, but a successful runtime apply must be committed to Desired and Live atomically.
- Successful runtime apply must update Live runtime artifacts to match the compiled Desired result, so service restarts and host reboots preserve runtime-applied changes.
- New runtime operations must document the Xray API capability they rely on and the verification path used to prove the running state before Desired and Live are committed.
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
- Windows Vagrant + SSH is flaky: a single SSH command may hang indefinitely even when a timeout is configured (Win10/Win11 host + Paramiko/Testinfra).
  - Mitigation: treat long-running SSH commands as hung; reconnect and retry once, and if still stuck, `vagrant reload --provision <machine>`.
  - If SSH becomes unstable mid-run, run two independent SSH probes in parallel (server + client) to "unstick" the VM/network, then retry.
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
- See `commands_map` when checking flag aliases.
- When changing CLI commands, flags, help, or command metadata, update `commands_map` with `make command-map`; do not edit generated command maps by hand.
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
