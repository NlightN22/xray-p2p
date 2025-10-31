$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
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

    $installed = choco.exe list --local-only $Package --limit-output |
        ForEach-Object { ($_ -split '\|')[1] } |
        Where-Object { $_ }

    if ($installed -and (-not $Version -or $installed -eq $Version)) {
        Write-Info "Chocolatey package '$Package' already installed ($installed)."
        return
    }

    $arguments = @("upgrade", $Package, "--yes", "--no-progress")
    if ($Version) {
        $arguments += @("--version", $Version)
    }

    Write-Info "Installing Chocolatey package '$Package'..."
    choco.exe @arguments | Write-Host
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
    Write-Info "Go toolchain ready: $goVersionOutput"
}

function Ensure-XrayCore {
    $installDir = "C:\tools\xray"
    $xrayExe = Join-Path $installDir "xray.exe"
    if (Test-Path $xrayExe) {
        Write-Info "xray-core already installed at $xrayExe"
        return
    }

    Write-Info "Installing xray-core..."
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir | Out-Null
    }

    $archivePath = Join-Path $env:TEMP "xray-core.zip"
    $downloadUrl = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"

    Write-Info "Downloading xray-core from $downloadUrl"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing

    Write-Info "Extracting xray-core archive..."
    if (Test-Path "$installDir\*") {
        Remove-Item -Path "$installDir\*" -Recurse -Force
    }
    Expand-Archive -Path $archivePath -DestinationPath $installDir -Force
    Remove-Item $archivePath -Force

    $xrayExeCandidate = Get-ChildItem -Path $installDir -Filter "xray.exe" -Recurse | Select-Object -First 1
    if (-not $xrayExeCandidate) {
        throw "Failed to locate xray.exe after extraction."
    }

    Copy-Item -Path $xrayExeCandidate.FullName -Destination $xrayExe -Force

    if (-not ($env:Path -split ';' | Where-Object { $_ -eq $installDir })) {
        Write-Info "Adding $installDir to the system PATH."
        $newPath = "$installDir;$($env:Path)"
        [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)
        $env:Path = $newPath
    }

    Write-Info "xray-core installed at $xrayExe"
}

function Build-Xp2p {
    $sourceRoot = "C:\xp2p"
    $xp2pDir = "C:\tools\xp2p"
    $xp2pExe = Join-Path $xp2pDir "xp2p.exe"

    if (-not (Test-Path $sourceRoot)) {
        throw "Shared folder '$sourceRoot' not mounted. Ensure Vagrant synced folders are available."
    }

    if (-not (Test-Path $xp2pDir)) {
        New-Item -ItemType Directory -Path $xp2pDir | Out-Null
    }

    Write-Info "Building xp2p binary..."
    Push-Location $sourceRoot
    try {
        & go.exe version | Write-Host
        & go.exe build -o $xp2pExe .\go\cmd\xp2p | Write-Host
    }
    finally {
        Pop-Location
    }

    if (-not (Test-Path $xp2pExe)) {
        throw "xp2p build failed - executable not found at $xp2pExe"
    }

    if (-not ($env:Path -split ';' | Where-Object { $_ -eq $xp2pDir })) {
        Write-Info "Adding $xp2pDir to the system PATH."
        $newPath = "$xp2pDir;$($env:Path)"
        [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)
        $env:Path = $newPath
    }

    Write-Info "xp2p built successfully at $xp2pExe"
}

function Ensure-OpenSSH {
    $capability = Get-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0' -ErrorAction SilentlyContinue
    if ($capability -and $capability.State -eq 'Installed') {
        Write-Info "OpenSSH Server capability already installed."
    }
    else {
        Write-Info "Installing OpenSSH Server capability..."
        Add-WindowsCapability -Online -Name 'OpenSSH.Server~~~~0.0.1.0' | Out-Null
    }

    Write-Info "Configuring OpenSSH service..."
    Set-Service -Name sshd -StartupType Automatic
    if ((Get-Service -Name sshd).Status -ne 'Running') {
        Start-Service -Name sshd
    }

    if (-not (Get-NetFirewallRule -DisplayName "Vagrant OpenSSH" -ErrorAction SilentlyContinue)) {
        Write-Info "Creating firewall exception for OpenSSH on port 22."
        New-NetFirewallRule -DisplayName "Vagrant OpenSSH" -Name "VagrantOpenSSH" `
            -Direction Inbound -Action Allow -Protocol TCP -LocalPort 22 | Out-Null
    }
}

function Configure-WinRM {
    Write-Info "Ensuring WinRM is configured for remote automation..."
    winrm quickconfig -q
    winrm set winrm/config/service/auth '@{Basic="true"}' | Out-Null
    winrm set winrm/config/service '@{AllowUnencrypted="true"}' | Out-Null
    Set-Item -Path WSMan:\localhost\Service\AllowUnencrypted -Value $true
    Set-Item -Path WSMan:\localhost\Service\Auth\Basic -Value $true
}

function Run-SmokeTest {
    param(
        [switch] $Skip
    )

    if ($Skip) {
        Write-Info "Skipping smoke test as requested."
        return
    }

    $xp2pExe = Get-Command -Name xp2p.exe -ErrorAction SilentlyContinue
    if (-not $xp2pExe) {
        throw "xp2p.exe not found in PATH. Cannot run smoke test."
    }

    Write-Info "Running smoke test: xp2p ping 127.0.0.1"
    & $xp2pExe.Source ping 127.0.0.1 | Write-Host
}

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Go
Ensure-XrayCore
Build-Xp2p
Ensure-OpenSSH
Configure-WinRM
Run-SmokeTest -Skip:$false

Write-Info "Provisioning completed successfully."
