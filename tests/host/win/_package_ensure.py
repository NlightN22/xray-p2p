from collections.abc import Callable

from testinfra.host import Host


def set_msi_build_id(build_id: str | None) -> None:
    from . import env as _env

    _env._MSI_BUILD_ID = build_id


def ensure_msi_package(
    host: Host,
    *,
    machine: str | None = None,
    reconnect: Callable[[], Host] | None = None,
) -> str:
    from . import env as _env

    if _env._MSI_CACHE_PATH_X64 and _env.path_exists(host, _env._MSI_CACHE_PATH_X64):
        return _env._MSI_CACHE_PATH_X64

    if machine is None:
        host = _env.ensure_project_synced(host)
    else:
        if reconnect is None:
            reconnect = lambda: _env.get_ssh_host(machine)
        host = _env.ensure_project_synced(host, machine=machine, reconnect=reconnect)
    path = _env._build_msi_package(
        host,
        architecture="amd64",
        cache_dir=_env.MSI_CACHE_DIR_X64,
        wix_source=r"installer\wix\xp2p.wxs",
    )
    _env._MSI_CACHE_PATH_X64 = path
    return path


def ensure_msi_package_x86(
    host: Host,
    *,
    machine: str | None = None,
    reconnect: Callable[[], Host] | None = None,
) -> str:
    from . import env as _env

    if _env._MSI_CACHE_PATH_X86 and _env.path_exists(host, _env._MSI_CACHE_PATH_X86):
        return _env._MSI_CACHE_PATH_X86

    if machine is None:
        host = _env.ensure_project_synced(host)
    else:
        if reconnect is None:
            reconnect = lambda: _env.get_ssh_host(machine)
        host = _env.ensure_project_synced(host, machine=machine, reconnect=reconnect)
    script = r"""
$ErrorActionPreference = 'SilentlyContinue'
if (-not (Get-Command -Name go.exe -ErrorAction SilentlyContinue)) {
    exit 0
}
& go.exe clean -cache -testcache 2>$null | Out-Null
exit 0
"""
    _env.run_powershell(host, script, timeout=120, label="go_clean_cache")
    path = _env._build_msi_package(
        host,
        architecture="x86",
        cache_dir=_env.MSI_CACHE_DIR_X86,
        wix_source=r"installer\wix\xp2p-x86.wxs",
    )
    _env._MSI_CACHE_PATH_X86 = path
    return path

