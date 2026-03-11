$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

function Wait-TcpPort {
    param(
        [Parameter(Mandatory = $true)]
        [string] $TargetHost,

        [Parameter(Mandatory = $true)]
        [int] $Port,

        [int] $TimeoutSeconds = 20,
        [int] $ProbeIntervalMilliseconds = 500
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $task = $client.ConnectAsync($TargetHost, $Port)
            if ($task.Wait($ProbeIntervalMilliseconds) -and $client.Connected) {
                return $true
            }
        }
        catch {
            # ignore and retry
        }
        finally {
            $client.Dispose()
        }

        Start-Sleep -Milliseconds $ProbeIntervalMilliseconds
    }

    return $false
}



function Ensure-IsElevated {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    $isElevated = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isElevated) {
        throw "Provisioning requires an elevated PowerShell session. Please rerun with Administrator privileges."
    }
}

function Ensure-Chocolatey {
    if (Get-Command -Name choco.exe -ErrorAction SilentlyContinue) {
        return
    }

    Write-Info "Chocolatey not detected. Installing Chocolatey..."
    Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
}

function Ensure-ChocoPackage {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Package,

        [string] $Version
    )

    $installArgs = @("install", $Package, "--yes", "--no-progress")
    if ($Version) {
        $installArgs += @("--version", $Version)
    }

    if (-not (choco list --local-only $Package | Select-String -Quiet "^$Package ")) {
        Write-Info ("Installing Chocolatey package '{0}' (version: {1})" -f $Package, $Version)
        choco $installArgs | Write-Host
    }
    else {
        Write-Info "Chocolatey package '$Package' already installed."
    }
}

function Disable-IdleSleepAndHibernate {
    Write-Info "Disabling sleep/hibernate and idle timeouts (AC/DC)"

    try {
        powercfg /hibernate off | Out-Null
        Write-Info "Hibernation disabled."
    }
    catch {
        Write-Info ("Failed to disable hibernation: {0}" -f $_.Exception.Message)
    }

    $commands = @(
        @('/x','-standby-timeout-ac','0'),
        @('/x','-standby-timeout-dc','0'),
        @('/x','-hibernate-timeout-ac','0'),
        @('/x','-hibernate-timeout-dc','0'),
        @('/x','-monitor-timeout-ac','0'),
        @('/x','-monitor-timeout-dc','0')
    )

    foreach ($cmd in $commands) {
        try {
            powercfg @cmd | Out-Null
        }
        catch {
            Write-Info ("powercfg {0} failed: {1}" -f ($cmd -join ' '), $_.Exception.Message)
        }
    }

    try {
        powercfg /setactive SCHEME_MIN | Out-Null
        Write-Info "Power scheme set to High performance."
    }
    catch {
        Write-Info ("Failed to set High performance scheme: {0}" -f $_.Exception.Message)
    }
}

function Disable-WindowsAutoUpdate {
    Write-Info "Disabling Windows Update automatic checks."

    $policyPath = "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU"
    try {
        if (-not (Test-Path $policyPath)) {
            New-Item -Path $policyPath -Force | Out-Null
        }
        New-ItemProperty -Path $policyPath -Name "NoAutoUpdate" -Value 1 -PropertyType DWord -Force | Out-Null
        New-ItemProperty -Path $policyPath -Name "AUOptions" -Value 2 -PropertyType DWord -Force | Out-Null
    }
    catch {
        Write-Info ("Failed to update Windows Update policy registry: {0}" -f $_.Exception.Message)
    }

    $services = @("wuauserv", "UsoSvc")
    foreach ($service in $services) {
        try {
            $svc = Get-Service -Name $service -ErrorAction Stop
            if ($svc.Status -ne "Stopped") {
                Stop-Service -Name $service -Force -ErrorAction Stop
            }
            Set-Service -Name $service -StartupType Disabled -ErrorAction Stop
        }
        catch {
            Write-Info ("Failed to disable service {0}: {1}" -f $service, $_.Exception.Message)
        }
    }
}

function Ensure-Go {
    $goVersion = $env:XP2P_GO_VERSION
    if (-not $goVersion) {
        # Default to the latest version from Chocolatey if not specified.
        $goVersion = $null
    }

    Ensure-ChocoPackage -Package "golang" -Version $goVersion

    if (-not (Get-Command -Name go.exe -ErrorAction SilentlyContinue)) {
        $goBinPaths = @(
            "C:\Program Files\Go\bin",
            "C:\tools\go\bin"
        )
        foreach ($path in $goBinPaths) {
            if (Test-Path $path) {
                Write-Info "Adding Go binary path '$path' to the current session PATH."
                $env:Path = "$path;$env:Path"
                break
            }
        }
    }

    $goVersionOutput = & go.exe version
    Write-Info ("Go toolchain ready: {0}" -f $goVersionOutput)
}

function Ensure-WiX {
    $wixVersion = $env:XP2P_WIX_VERSION
    if (-not $wixVersion) {
        $wixVersion = $null
    }

    $wixDirectories = Get-ChildItem "C:\Program Files (x86)" -Filter "WiX Toolset*" -Directory -ErrorAction SilentlyContinue
    if (-not $wixDirectories) {
        Ensure-ChocoPackage -Package "wixtoolset" -Version $wixVersion
        $wixDirectories = Get-ChildItem "C:\Program Files (x86)" -Filter "WiX Toolset*" -Directory -ErrorAction SilentlyContinue
    }

    if (-not $wixDirectories) {
        throw "WiX Toolset installation directory not found even after installation."
    }

    $latest = $wixDirectories | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    $binPath = Join-Path $latest.FullName "bin"
    $candlePath = Join-Path $binPath "candle.exe"
    $lightPath = Join-Path $binPath "light.exe"

    if (-not (Test-Path $candlePath)) {
        throw "candle.exe not found in '$binPath'."
    }
    if (-not (Test-Path $lightPath)) {
        throw "light.exe not found in '$binPath'."
    }

    Write-Info ("WiX Toolset ready: {0}" -f $latest.FullName)
}

Write-Info "Provisioning role detected."

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Go
Ensure-WiX
Disable-WindowsAutoUpdate
Disable-IdleSleepAndHibernate
Write-Info "Network configuration handled by network_setup.ps1."
Write-Info "Provisioning completed successfully."
