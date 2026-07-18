from testinfra.host import Host


def _cleanup_orphaned_xp2p_msi(host: Host) -> None:
    from . import env as _env

    script = r"""
$ErrorActionPreference = 'SilentlyContinue'
$services = @('xp2p-client', 'xp2p-server')
foreach ($svc in $services) {
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service) {
        Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
        sc.exe delete $svc | Out-Null
    }
}
Get-Process -Name xp2p,xray,ui-xp2p,msiexec -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

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
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall'
)
$productKeys = New-Object System.Collections.Generic.List[string]
foreach ($root in $roots) {
    if (-not (Test-Path $root)) { continue }
    Get-ChildItem -Path $root -ErrorAction SilentlyContinue | ForEach-Object {
        $props = Get-ItemProperty -Path $_.PSPath -ErrorAction SilentlyContinue
        $name = $props.DisplayName
        if (-not $name) { $name = $props.ProductName }
        if ($name -and $name -match $productNamePattern) {
            Remove-Item -Path $_.PSPath -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

$installerRoots = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products'
)
foreach ($root in $installerRoots) {
    if (-not (Test-Path $root)) { continue }
    $children = Get-ChildItem -Path $root -ErrorAction SilentlyContinue
    foreach ($child in $children) {
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
    if (-not (Test-Path $root)) { continue }
    Get-ChildItem -Path $root -ErrorAction SilentlyContinue | ForEach-Object {
        $props = Get-ItemProperty -Path $_.PSPath -ErrorAction SilentlyContinue
        $name = $props.ProductName
        if (-not $name) { $name = $props.DisplayName }
        if (($name -and $name -match $productNamePattern) -or ($productKeys -contains $_.PSChildName)) {
            Remove-Item -Path $_.PSPath -Recurse -Force -ErrorAction SilentlyContinue
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
exit 0
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
