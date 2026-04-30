import os
from pathlib import Path

from testinfra.host import Host


def _set_install_paths_from_exe(exe_path: Path) -> Path:
    from . import env as _env

    install_dir = exe_path.parent
    if install_dir.name.lower() == "bin" and install_dir.parent:
        install_dir = install_dir.parent
    _env.PROGRAM_FILES_INSTALL_DIR = install_dir
    if "XP2P_CONFIG_ROOT" not in os.environ:
        _env.CONFIG_ROOT = _env.PROGRAM_DATA_ROOT
    if "XP2P_LOG_ROOT" not in os.environ:
        _env.LOGS_DIR = _env.CONFIG_ROOT / "logs"
    _env.XP2P_EXE = exe_path
    return _env.PROGRAM_FILES_INSTALL_DIR


def _detect_xp2p_exe(host: Host) -> Path | None:
    from . import env as _env

    candidates = [
        _env.PROGRAM_FILES_INSTALL_DIR / "xp2p.exe",
        _env.PROGRAM_FILES_INSTALL_DIR / "bin" / "xp2p.exe",
        _env.PROGRAM_FILES_X86_INSTALL_DIR / "xp2p.exe",
        _env.PROGRAM_FILES_X86_INSTALL_DIR / "bin" / "xp2p.exe",
    ]
    for candidate in candidates:
        if _env.path_exists(host, candidate):
            return candidate

    install_root = _query_install_location(host)
    if install_root:
        for candidate in (install_root / "xp2p.exe", install_root / "bin" / "xp2p.exe"):
            if _env.path_exists(host, candidate):
                return candidate

    search_roots = [
        Path(r"C:\Program Files"),
        Path(r"C:\Program Files (x86)"),
        Path(r"C:\ProgramData"),
    ]
    roots = ", ".join(_env.ps_quote(str(root)) for root in search_roots)
    script = f"""
$ErrorActionPreference = 'Stop'
$roots = @({roots})
foreach ($root in $roots) {{
    if (-not (Test-Path $root)) {{
        continue
    }}
    $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if ($found) {{
        Write-Output $found
        exit 0
    }}
}}
exit 3
"""
    result = _env.run_powershell(host, script, label="detect_xp2p_exe_scan")
    if result.rc != 0:
        return _search_user_programs(host)
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return _search_user_programs(host)
    return Path(value[-1].strip())


def find_xp2p_exe(host: Host, hint_path: Path | None = None) -> Path | None:
    from . import env as _env

    result = _env.run_guest_script(
        host,
        "scripts/find_xp2p_exe.ps1",
        HintPath=str(hint_path) if hint_path else "",
    )
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


def _search_user_programs(host: Host) -> Path | None:
    from . import env as _env

    script = """
$ErrorActionPreference = 'Stop'
$usersRoot = 'C:\\Users'
if (-not (Test-Path $usersRoot)) {
    exit 3
}
$users = Get-ChildItem -Path $usersRoot -Directory -ErrorAction SilentlyContinue
foreach ($user in $users) {
    $root = Join-Path $user.FullName 'AppData\\Local\\Programs'
    if (-not (Test-Path $root)) {
        continue
    }
    $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if ($found) {
        Write-Output $found
        exit 0
    }
}
exit 3
"""
    result = _env.run_powershell(host, script, label="search_user_programs")
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


def _query_install_location(host: Host) -> Path | None:
    from . import env as _env

    script = """
$ErrorActionPreference = 'SilentlyContinue'
$roots = @(
    'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKLM:\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*'
)
$items = Get-ItemProperty -Path $roots | Where-Object {
    $_.DisplayName -and $_.DisplayName -like 'xp2p*'
}
foreach ($item in $items) {
    if ($item.InstallLocation) {
        Write-Output $item.InstallLocation
        exit 0
    }
}
exit 3
"""
    result = _env.run_powershell(host, script, label="query_install_location")
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


def get_program_files_install_dir(host: Host) -> Path:
    detected = _detect_xp2p_exe(host)
    if detected is not None:
        return _set_install_paths_from_exe(detected)
    from . import env as _env

    return _env.PROGRAM_FILES_INSTALL_DIR

