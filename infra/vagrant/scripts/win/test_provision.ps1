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

function Ensure-Sysinternals {
    $sysinternalsVersion = $env:XP2P_SYSINTERNALS_VERSION
    if (-not $sysinternalsVersion) {
        $sysinternalsVersion = $null
    }

    Ensure-ChocoPackage -Package "sysinternals" -Version $sysinternalsVersion
}

Write-Info "Provisioning role detected (test tooling)."

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Sysinternals

Write-Info "Provisioning completed successfully."
