[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [switch] $Purge = $false,
    [switch] $Quiet = $true,
    [string] $LogPath = 'C:\Windows\Temp\xp2p-uninstall.log'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Info {
    param([Parameter(Mandatory = $true)][string] $Message)
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    Write-Host ("==> [{0}] {1}" -f $ts, $Message)
}

function Get-InstalledXp2pProductCodes {
    $codes = New-Object System.Collections.Generic.List[string]
    $roots = @(
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    foreach ($root in $roots) {
        $items = Get-ItemProperty -Path $root -ErrorAction SilentlyContinue | Where-Object {
            $_ -and $_.PSObject.Properties.Match('DisplayName').Count -gt 0 -and $_.DisplayName -and $_.DisplayName -match '^xp2p(\s|$)'
        }
        foreach ($item in $items) {
            $code = $item.PSChildName
            if ($code -and $code -match '^\{[0-9A-Fa-f-]+\}$') {
                $codes.Add($code) | Out-Null
            }
        }
    }
    return @($codes | Select-Object -Unique)
}

function Remove-Xp2pArpEntries {
    param([string[]] $ProductCodes)

    $codes = @($ProductCodes | Where-Object { $_ -and $_ -match '^\{[0-9A-Fa-f-]+\}$' } | Select-Object -Unique)
    $roots = @(
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )

    foreach ($root in $roots) {
        $items = Get-ItemProperty -Path $root -ErrorAction SilentlyContinue | Where-Object {
            $_ -and $_.PSObject.Properties.Match('DisplayName').Count -gt 0 -and $_.DisplayName -and $_.DisplayName -match '^xp2p(\s|$)'
        }
        foreach ($item in $items) {
            $matchesCode = $false
            if ($codes.Count -gt 0) {
                $keyName = $item.PSChildName
                if ($keyName -and ($codes -contains $keyName)) {
                    $matchesCode = $true
                }
            }

            if (-not $matchesCode -and $codes.Count -gt 0) {
                continue
            }

            $path = $item.PSPath
            if ($path -and (Test-Path $path)) {
                if ($PSCmdlet.ShouldProcess($path, 'Remove ARP uninstall entry')) {
                    Remove-Item -Path $path -Recurse -Force -ErrorAction SilentlyContinue
                }
            }
        }
    }
}

function Stop-Xp2pProcessesAndServices {
    $services = @('xp2p-client', 'xp2p-server')
    foreach ($svc in $services) {
        $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
        if ($service -and $service.Status -ne 'Stopped') {
            if ($PSCmdlet.ShouldProcess("service:$svc", 'Stop')) {
                Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
            }
        }
    }
    if ($PSCmdlet.ShouldProcess('processes:xp2p,xray,ui-xp2p', 'Stop')) {
        Get-Process -Name xp2p,xray,ui-xp2p -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    }
}

function Remove-Xp2pAutostartAndShortcuts {
    foreach ($sid in (Get-ChildItem Registry::HKEY_USERS -ErrorAction SilentlyContinue | Select-Object -ExpandProperty PSChildName)) {
        $runKey = "Registry::HKEY_USERS\$sid\Software\Microsoft\Windows\CurrentVersion\Run"
        if (Test-Path $runKey) {
            if ($PSCmdlet.ShouldProcess($runKey, 'Remove ui-xp2p autostart')) {
                Remove-ItemProperty -Path $runKey -Name 'ui-xp2p' -ErrorAction SilentlyContinue
            }
        }
        $xp2pKey = "Registry::HKEY_USERS\$sid\Software\xp2p"
        if (Test-Path $xp2pKey) {
            if ($PSCmdlet.ShouldProcess($xp2pKey, 'Remove xp2p registry key')) {
                Remove-Item -Path $xp2pKey -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }

    Get-ChildItem -Path 'C:\Users' -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        $userDir = $_.FullName
        $desktopShortcut = Join-Path $userDir 'Desktop\ui-xp2p.lnk'
        $startMenuShortcut = Join-Path $userDir 'AppData\Roaming\Microsoft\Windows\Start Menu\Programs\xp2p\ui-xp2p.lnk'
        if ($PSCmdlet.ShouldProcess($desktopShortcut, 'Remove shortcut')) {
            Remove-Item -Path $desktopShortcut -Force -ErrorAction SilentlyContinue
        }
        if ($PSCmdlet.ShouldProcess($startMenuShortcut, 'Remove shortcut')) {
            Remove-Item -Path $startMenuShortcut -Force -ErrorAction SilentlyContinue
        }
        $startMenuDir = Split-Path -Parent $startMenuShortcut
        if ($startMenuDir -and (Test-Path $startMenuDir)) {
            $remaining = Get-ChildItem -Path $startMenuDir -ErrorAction SilentlyContinue
            if (-not $remaining) {
                if ($PSCmdlet.ShouldProcess($startMenuDir, 'Remove empty folder')) {
                    Remove-Item -Path $startMenuDir -Force -ErrorAction SilentlyContinue
                }
            }
        }
    }
}

function Remove-Xp2pInstallDirs {
    $dirs = @(
        (Join-Path $env:ProgramFiles 'xp2p'),
        (Join-Path ${env:ProgramFiles(x86)} 'xp2p')
    ) | Where-Object { $_ -and $_ -ne '' } | Select-Object -Unique
    foreach ($dir in $dirs) {
        if (Test-Path $dir) {
            if ($PSCmdlet.ShouldProcess($dir, 'Remove directory')) {
                Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

function Remove-Xp2pServicesBestEffort {
    foreach ($svc in @('xp2p-client', 'xp2p-server')) {
        if ($PSCmdlet.ShouldProcess("service:$svc", 'Delete')) {
            sc.exe stop $svc | Out-Null
            sc.exe delete $svc | Out-Null
        }
    }
}

Stop-Xp2pProcessesAndServices

$codes = @(Get-InstalledXp2pProductCodes)
if ($codes.Count -eq 0) {
    Write-Info 'No installed xp2p MSI product codes found.'
    if ($Purge) {
        Write-Info 'Purging leftover autostart and install directories.'
        Remove-Xp2pAutostartAndShortcuts
        Remove-Xp2pInstallDirs
        Remove-Xp2pServicesBestEffort
    }
    exit 0
}

Write-Info ("Found installed xp2p products: {0}" -f ($codes -join ', '))

foreach ($code in $codes) {
    $args = @('/x', $code, '/norestart', '/l*v', $LogPath)
    if ($Quiet) {
        $args += '/qn'
    }
    Write-Info ("Running msiexec {0}" -f ($args -join ' '))
    if (-not $PSCmdlet.ShouldProcess("msi:$code", 'Uninstall')) {
        continue
    }
    $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList $args -Wait -PassThru
    Write-Info ("msiexec exit code: {0}" -f $proc.ExitCode)
    $successCodes = @(0, 1605, 1614, 3010)
    if ($successCodes -notcontains $proc.ExitCode) {
        throw "MSI uninstall failed with exit code $($proc.ExitCode). See $LogPath."
    }
}

if ($Purge) {
    Write-Info 'Purging leftover autostart and install directories.'
    Stop-Xp2pProcessesAndServices
    Remove-Xp2pAutostartAndShortcuts
    Remove-Xp2pInstallDirs
    Remove-Xp2pServicesBestEffort
    Remove-Xp2pArpEntries -ProductCodes $codes
}

Write-Info 'Done.'
