from pathlib import Path

from testinfra.host import Host


def _manual_install_from_msi_bin(host: Host) -> None:
    from . import env as _env

    install_dir = _env.PROGRAM_FILES_INSTALL_DIR
    src_root = Path(r"C:\xp2p\build\msi-bin")
    script = f"""
$ErrorActionPreference = 'Stop'
$src = {_env.ps_quote(str(src_root))}
$dst = {_env.ps_quote(str(install_dir))}
$xp2p = Join-Path $src 'xp2p.exe'
$bundle = Join-Path $src 'bundle'
$xray = Join-Path $bundle 'xray.exe'
$wintun = Join-Path $bundle 'wintun.dll'
if (-not (Test-Path $xp2p)) {{
    throw "Fallback install failed: $xp2p not found"
}}
if (-not (Test-Path $xray)) {{
    throw "Fallback install failed: $xray not found"
}}
if (-not (Test-Path $wintun)) {{
    throw "Fallback install failed: $wintun not found"
}}
if (-not (Test-Path $dst)) {{
    New-Item -ItemType Directory -Path $dst -Force | Out-Null
}}
$bin = Join-Path $dst 'bin'
if (-not (Test-Path $bin)) {{
    New-Item -ItemType Directory -Path $bin -Force | Out-Null
}}
Copy-Item -Path $xp2p -Destination (Join-Path $dst 'xp2p.exe') -Force
Copy-Item -Path $xray -Destination (Join-Path $bin 'xray.exe') -Force
Copy-Item -Path $wintun -Destination (Join-Path $bin 'wintun.dll') -Force
"""
    _env.run_powershell(host, script)

