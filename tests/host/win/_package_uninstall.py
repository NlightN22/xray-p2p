from pathlib import Path

from testinfra.host import Host


def uninstall_xp2p_from_msi(host: Host, msi_path: str | Path, *, purge_files: bool = True) -> None:
    from . import env as _env

    msi_str = _env.ps_quote(str(msi_path))
    install_dir = _env.ps_quote(str(_env.get_program_files_install_dir(host)))
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
$waitSeconds = 110
$services = @('xp2p-client', 'xp2p-server')
foreach ($svc in $services) {{
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service -and $service.Status -ne 'Stopped') {{
        Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
    }}
}}
Get-Process -Name xp2p,xray,ui-xp2p -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
$productCodes = @()
$roots = @(
    'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKLM:\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*'
)
foreach ($root in $roots) {{
    $items = Get-ItemProperty -Path $root -ErrorAction SilentlyContinue | Where-Object {{
        $_.DisplayName -and $_.DisplayName -match '^xp2p(\\s|$)'
    }}
    foreach ($item in $items) {{
        $code = $item.PSChildName
        if ($code -and $code -match '^\\{{[0-9A-Fa-f-]+\\}}$') {{
            $productCodes += $code
            continue
        }}
        $uninstall = $item.UninstallString
        if ($uninstall -and $uninstall -match '/X(\\{{[0-9A-Fa-f-]+\\}})') {{
            $productCodes += $matches[1]
        }}
    }}
}}
$productCodes = @($productCodes | Select-Object -Unique)

$productCode = $null
if ($productCodes.Count -gt 0) {{
    $productCode = [string]$productCodes[0]
}}

$arguments = $null
if ($productCode -and $productCode -match '^\\{{[0-9A-Fa-f-]+\\}}$') {{
    $arguments = @('/x', $productCode, '/qn', '/norestart')
}} else {{
    $arguments = @('/x', $msi, '/qn', '/norestart')
}}
$attempt = 0
do {{
    $attempt++
    $process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
    if (-not $process.WaitForExit($waitSeconds * 1000)) {{
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Write-Output "MSI ExitCode=124"
        exit 124
    }}
    Write-Output ("MSI ExitCode=" + $process.ExitCode)
    $successCodes = @(0, 1605, 1614, 3010)
    if ($successCodes -contains $process.ExitCode) {{
        break
    }}
    Start-Sleep -Seconds 2
}} while ($attempt -lt 2)
if ($successCodes -notcontains $process.ExitCode) {{
    exit $process.ExitCode
}}

foreach ($sid in (Get-ChildItem Registry::HKEY_USERS -ErrorAction SilentlyContinue | Select-Object -ExpandProperty PSChildName)) {{
    $runKey = "Registry::HKEY_USERS\\$sid\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
    if (Test-Path $runKey) {{
        Remove-ItemProperty -Path $runKey -Name 'ui-xp2p' -ErrorAction SilentlyContinue
    }}
    $xp2pKey = "Registry::HKEY_USERS\\$sid\\Software\\xp2p"
    if (Test-Path $xp2pKey) {{
        Remove-Item -Path $xp2pKey -Recurse -Force -ErrorAction SilentlyContinue
    }}
}}
"""
    if purge_files:
        script += f"""
if (Test-Path {install_dir}) {{
    Remove-Item {install_dir} -Force -Recurse -ErrorAction SilentlyContinue
}}
"""
    script += """
exit 0
"""
    result = _env.run_powershell(host, script, timeout=360, label="msi_uninstall")
    if result.rc != 0:
        stdout = result.stdout or ""
        if "MSI ExitCode=1601" in stdout:
            _env.remove_services(host, ["xp2p-client", "xp2p-server"])
            _env.remove_paths(
                host,
                [
                    _env.PROGRAM_FILES_INSTALL_DIR,
                    _env.PROGRAM_FILES_X86_INSTALL_DIR,
                    _env.PROGRAM_DATA_ROOT,
                ],
            )
            print("WARNING: MSI uninstall failed (1601); cleaned up xp2p artifacts manually.")
            return
        if (
            not _env.path_exists(host, _env.XP2P_EXE)
            and not _env.service_exists(host, "xp2p-client")
            and not _env.service_exists(host, "xp2p-server")
        ):
            print("WARNING: MSI uninstall reported failure, but xp2p artifacts are already removed.")
            return
        raise RuntimeError(
            "Failed to uninstall xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    _env.remove_services(host, ["xp2p-client", "xp2p-server"])
    if purge_files:
        _env.purge_xp2p_install(host, purge=True, label="msi_uninstall_purge")

