# Windows host test suite (`tests/host/win`)

This suite validates Windows-specific `xp2p` behavior end-to-end via SSH + guest PowerShell scripts.

## Contract (state machine)

Tests should follow the `desired → live → runtime OS-state` contract:

- Desired inputs live under `CONFIG_ROOT` (default: `C:\ProgramData\xp2p`), e.g. `xp2p-client.toml`.
- `xp2p apply` (or the service reconciler) compiles desired inputs into live artifacts atomically under `CONFIG_ROOT\.state\live\...`.
- Runtime checks should read live artifacts and OS state (routes, interfaces, Windows services, logs) and must not depend on desired inputs.
- On invalid desired inputs, apply may fail while keeping the last known good live artifacts; failures are recorded via `CONFIG_ROOT\.state\apply.error`.

## Layout

- `tests/host/win/flows/`: high-level flows (waiters, deploy helpers, Windows SCM helpers).
- `tests/host/win/assertions/`: state-machine-friendly assertions (prefer live artifacts + logs).
- `tests/host/win/diagnostics/`: unified data collection helpers (net dumps, remote reads).
- `tests/host/win/test_*.py`: thin scenario tests that compose flows + assertions.

## Inventory (grouped by feature)

MSI installer / packaging
- `tests/host/win/test_installer_msi.py`
- `tests/host/win/test_dual_install.py`

Client install & run
- `tests/host/win/test_client_install.py`

Server install & cert management
- `tests/host/win/test_server_install.py`

Service CLI / SCM behavior
- `tests/host/win/test_service_cli.py`
- `tests/host/win/test_service_restart_log.py`

Users / admin rights
- `tests/host/win/test_server_users.py`
- `tests/host/win/test_server_admin.py`

Deploy flows
- `tests/host/win/test_client_deploy.py`
- `tests/host/win/tun_full_deploy.py`

Full tunnel (TUN) / connectivity
- `tests/host/win/test_tun_mode_full.py`
- `tests/host/win/test_tunnel_client_server.py`
- `tests/host/win/test_send_through.py`
- `tests/host/win/test_ping_client2server.py`

Redirect / routing
- `tests/host/win/test_client_redirect.py`
- `tests/host/win/test_redirect_routes_os.py`
- `tests/host/win/test_tunnel_redirect.py`

UI integration
- `tests/host/win/test_xp2p_ui_integration.py`

Xray version pinning
- `tests/host/win/test_xray_pinned_version.py`

## Logging

Recommended run pattern (stores output in `.logs/tests/`):

```powershell
$ts = Get-Date -Format 'yyyyMMdd-HHmmss'
pytest tests\host\win\test_xp2p_ui_integration.py -vv -s 2>&1 |
  Tee-Object -FilePath (".\.logs\tests\pytest-win-$ts.log")
```

## Migration plan (2–4 iterations)

1) Extract shared utilities (apply waiters, remote reads, dumps) without changing scenarios.
2) Pull high-level flows out of the largest test files; keep `test_*.py` scenario-focused.
3) Unify state-machine checks inside Windows suite (apply/live/service log patterns) and remove duplication.
4) Stabilize timing/sync (polling/backoff, explicit markers, failure bundles) where flakes are observed.

