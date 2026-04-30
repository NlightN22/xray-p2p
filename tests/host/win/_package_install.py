from pathlib import Path

from testinfra.host import Host


def install_xp2p_from_msi(host: Host, msi_path: str | Path) -> None:
    from . import env as _env

    msi_str = _env.ps_quote(str(msi_path))
    log_path = _env.ps_quote(r"C:\xp2p\build\logs\win\msi-install.log")
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
if (-not (Test-Path $msi)) {{
    throw "MSI package not found at $msi"
}}
$policyRoots = @(
    'HKLM:\\Software\\Policies\\Microsoft\\Windows\\Installer',
    'HKCU:\\Software\\Policies\\Microsoft\\Windows\\Installer'
)
foreach ($policyRoot in $policyRoots) {{
    if (Test-Path $policyRoot) {{
        Set-ItemProperty -Path $policyRoot -Name 'DisableMSI' -Value 0 -ErrorAction SilentlyContinue
    }}
}}
$svc = Get-Service -Name 'msiserver' -ErrorAction SilentlyContinue
if ($svc) {{
    if ($svc.StartType -eq 'Disabled') {{
        Set-Service -Name 'msiserver' -StartupType Manual -ErrorAction SilentlyContinue
    }}
    if ($svc.Status -ne 'Running') {{
        Start-Service -Name 'msiserver' -ErrorAction SilentlyContinue
    }}
}}
$logPath = {log_path}
$logDir = Split-Path -Parent $logPath
if ($logDir -and -not (Test-Path $logDir)) {{
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}}
if (Test-Path $logPath) {{
    Remove-Item $logPath -Force -ErrorAction SilentlyContinue
}}
$arguments = @('/i', $msi, '/qn', '/norestart', '/l*v', $logPath, 'XP2P_SKIP_SERVICE_START=1')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
if (-not $process.WaitForExit(300000)) {{
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    exit 124
}}
if ($process.ExitCode -ne 0) {{
    Write-Output ("MSI ExitCode=" + $process.ExitCode)
    exit $process.ExitCode
}}
exit 0
"""
    result = _env.run_powershell(host, script, timeout=420, label="msi_install")
    if result.rc != 0:
        log_path_obj = Path(r"C:\xp2p\build\logs\win\msi-install.log")
        log_tail = _read_msi_log_tail(host, log_path_obj)
        log_context = _read_msi_failure_context(host, log_path_obj)
        stdout = result.stdout or ""
        if "MSI ExitCode=1601" in stdout:
            _env.run_powershell(
                host,
                """
$ErrorActionPreference = 'SilentlyContinue'
foreach ($policyRoot in @(
    'HKLM:\\Software\\Policies\\Microsoft\\Windows\\Installer',
    'HKCU:\\Software\\Policies\\Microsoft\\Windows\\Installer'
)) {
    if (Test-Path $policyRoot) {
        Set-ItemProperty -Path $policyRoot -Name 'DisableMSI' -Value 0 -ErrorAction SilentlyContinue
    }
}
sc.exe config msiserver start= demand | Out-Null
Start-Service -Name 'msiserver' -ErrorAction SilentlyContinue | Out-Null
Start-Process -FilePath 'msiexec.exe' -ArgumentList '/unregister' -Wait -ErrorAction SilentlyContinue | Out-Null
Start-Process -FilePath 'msiexec.exe' -ArgumentList '/regserver' -Wait -ErrorAction SilentlyContinue | Out-Null
""",
                timeout=120,
            )
            result = _env.run_powershell(host, script, timeout=420, label="msi_install_retry")
            if result.rc == 0:
                return
            raise _env.MsiServiceUnavailable(
                "Windows Installer service is unavailable (MSI ExitCode=1601)."
            )
        if "MSI ExitCode=1603" in stdout:
            _env._cleanup_orphaned_xp2p_msi(host)
            result = _env.run_powershell(host, script, timeout=420, label="msi_install_retry")
            if result.rc == 0:
                return
        raise RuntimeError(
            "Failed to install xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            f"{log_context}{log_tail}"
        )


def _read_msi_log_tail(host: Host, path: Path, lines: int = 200) -> str:
    from . import env as _env

    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
$content = Get-Content -Path $target -Tail {lines}
$content
exit 0
"""
    result = _env.run_powershell(host, script, label="msi_log_tail")
    if result.rc != 0:
        return "\nMSI log tail: <missing>\n"
    tail = (result.stdout or "").strip()
    if not tail:
        return "\nMSI log tail: <empty>\n"
    return "\nMSI log tail:\n" + tail + "\n"


def _read_msi_failure_context(host: Host, path: Path) -> str:
    from . import env as _env

    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
$lines = Get-Content -Path $target
$failIndex = -1
for ($i = $lines.Count - 1; $i -ge 0; $i--) {{
    if ($lines[$i] -match 'Return value 3') {{
        $failIndex = $i
        break
    }}
}}
if ($failIndex -lt 0) {{
    exit 0
}}
$startIndex = [Math]::Max(0, $failIndex - 40)
$lines[$startIndex..$failIndex]
exit 0
"""
    result = _env.run_powershell(host, script, label="msi_log_context")
    if result.rc != 0:
        return ""
    context = (result.stdout or "").strip()
    if not context:
        return ""
    return "\nMSI failure context:\n" + context + "\n"

