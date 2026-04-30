from testinfra.host import Host


def _cleanup_orphaned_xp2p_msi(host: Host) -> None:
    from . import env as _env

    script = r"""
$ErrorActionPreference = 'Stop'
$services = @('xp2p-client', 'xp2p-server')
foreach ($svc in $services) {
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service -and $service.Status -ne 'Stopped') {
        Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
    }
}
Get-Process -Name xp2p,xray,ui-xp2p -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

foreach ($sid in (Get-ChildItem Registry::HKEY_USERS -ErrorAction SilentlyContinue | Select-Object -ExpandProperty PSChildName)) {
    $runKey = "Registry::HKEY_USERS\$sid\Software\Microsoft\Windows\CurrentVersion\Run"
    if (Test-Path $runKey) {
        Remove-ItemProperty -Path $runKey -Name 'ui-xp2p' -ErrorAction SilentlyContinue
    }
    $xp2pKey = "Registry::HKEY_USERS\$sid\Software\xp2p"
    if (Test-Path $xp2pKey) {
        Remove-Item -Path $xp2pKey -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$profileRoots = @('C:\Users')
foreach ($root in $profileRoots) {
    Get-ChildItem -Path $root -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        $userDir = $_.FullName
        $desktopShortcut = Join-Path $userDir 'Desktop\ui-xp2p.lnk'
        $startMenuShortcut = Join-Path $userDir 'AppData\Roaming\Microsoft\Windows\Start Menu\Programs\xp2p\ui-xp2p.lnk'
        Remove-Item -Path $desktopShortcut -Force -ErrorAction SilentlyContinue
        Remove-Item -Path $startMenuShortcut -Force -ErrorAction SilentlyContinue
        $startMenuDir = Split-Path -Parent $startMenuShortcut
        if ($startMenuDir -and (Test-Path $startMenuDir)) {
            $remaining = Get-ChildItem -Path $startMenuDir -ErrorAction SilentlyContinue
            if (-not $remaining) {
                Remove-Item -Path $startMenuDir -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

$productNamePattern = '^xp2p'
$roots = @(
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
$items = Get-ItemProperty -Path $roots -ErrorAction SilentlyContinue | Where-Object {
    $_.DisplayName -and $_.DisplayName -match $productNamePattern
}

foreach ($item in $items) {
    $code = $item.PSChildName
    if ($code -and $code -match '^\{[0-9A-Fa-f-]+\}$') {
        $args = @('/x', $code, '/qn', '/norestart')
        $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList $args -PassThru
        $proc.WaitForExit(120000) | Out-Null
        if (-not $proc.HasExited) {
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        }
        continue
    }
    $cmd = $null
    if ($item.QuietUninstallString) {
        $cmd = $item.QuietUninstallString
    } elseif ($item.UninstallString) {
        $cmd = $item.UninstallString
    }
    if ($cmd) {
        $cmd = $cmd -replace '/I', '/X'
        Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', $cmd) -Wait -ErrorAction SilentlyContinue | Out-Null
    }
}

$deadline = (Get-Date).AddSeconds(120)
$installerRoots = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products'
)
$productKeys = New-Object System.Collections.Generic.List[string]
foreach ($root in $installerRoots) {
    if ((Get-Date) -gt $deadline) { break }
    $children = Get-ChildItem -Path $root -ErrorAction SilentlyContinue
    foreach ($child in $children) {
        if ((Get-Date) -gt $deadline) { break }
        $propsPath = Join-Path $child.PSPath 'InstallProperties'
        $props = Get-ItemProperty -Path $propsPath -ErrorAction SilentlyContinue
        if (-not $props) { continue }
        $name = $props.DisplayName
        if (-not $name) { $name = $props.ProductName }
        if ($name -and $name -match $productNamePattern) {
            $productKeys.Add($child.PSChildName) | Out-Null
            Remove-Item -Path $child.PSPath -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}
$classRoots = @(
    'HKLM:\SOFTWARE\Classes\Installer\Products',
    'HKLM:\SOFTWARE\WOW6432Node\Classes\Installer\Products'
)
foreach ($root in $classRoots) {
    foreach ($key in $productKeys) {
        $target = Join-Path $root $key
        if (Test-Path $target) {
            Remove-Item -Path $target -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

$dirs = @(
    'C:\ProgramData\xp2p',
    'C:\Program Files\xp2p',
    'C:\Program Files (x86)\xp2p'
)
foreach ($dir in $dirs) {
    if (Test-Path $dir) {
        Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
"""
    _env.run_powershell(host, script, timeout=300, label="msi_cleanup_orphans")
    purge_xp2p_install(host, purge=True, label="msi_cleanup_orphans_purge")


def purge_xp2p_install(host: Host, *, purge: bool = True, label: str = "xp2p_purge") -> None:
    from . import env as _env

    script_path = _env.ps_quote(str(_env.XP2P_UNINSTALL_SCRIPT))
    flags = "-Quiet" + (" -Purge" if purge else "")
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$path = {script_path}
if (Test-Path $path) {{
    & $path {flags} | Out-Null
}}
exit 0
"""
    _env.run_powershell(host, script, timeout=240, label=label)

