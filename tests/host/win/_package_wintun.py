from pathlib import Path

from testinfra.host import Host


def ensure_wintun_dll(host: Host, install_dir: Path | None = None) -> Path:
    from . import env as _env

    if install_dir is None:
        install_dir = _env.get_program_files_install_dir(host)
    dest = Path(install_dir) / "bin" / "wintun.dll"
    if _env.path_exists(host, dest):
        return dest
    script = f"""
$ErrorActionPreference = 'Stop'
$dest = {_env.ps_quote(str(dest))}
$binDir = Split-Path -Parent $dest
if ($binDir -and -not (Test-Path $binDir)) {{
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
}}
$sources = @(
    {_env.ps_quote(str(_env.WINTUN_DLL_SOURCE_MSI_BIN_X64))},
    {_env.ps_quote(str(_env.WINTUN_DLL_SOURCE_BUNDLE_X64))}
)
$src = $null
foreach ($candidate in $sources) {{
    if (Test-Path $candidate) {{
        $src = $candidate
        break
    }}
}}
if (-not $src) {{
    throw "wintun.dll not found in any known source path."
}}
Copy-Item -Path $src -Destination $dest -Force
"""
    result = _env.run_powershell(host, script, timeout=60, label="ensure_wintun_dll")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to place wintun.dll into the install directory.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    if not _env.path_exists(host, dest):
        raise RuntimeError(f"wintun.dll missing after copy: {dest}")
    return dest

