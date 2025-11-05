$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

function Ensure-OpenSshFeature {
    $capabilities = @(
        "OpenSSH.Client~~~~0.0.1.0",
        "OpenSSH.Server~~~~0.0.1.0"
    )

    foreach ($capability in $capabilities) {
        $current = Get-WindowsCapability -Online -Name $capability -ErrorAction SilentlyContinue
        if ($current -and $current.State -eq "Installed") {
            Write-Info ("Windows capability '{0}' already installed." -f $capability)
            continue
        }

        Write-Info ("Installing Windows capability '{0}'" -f $capability)
        Add-WindowsCapability -Online -Name $capability -ErrorAction Stop | Out-Null
    }
}

function Register-SshdServiceManual {
    $exePath = Join-Path $env:SystemRoot "System32\OpenSSH\sshd.exe"
    if (-not (Test-Path $exePath)) {
        throw "Cannot manually register sshd: executable not found at $exePath"
    }

    $configDir = Join-Path $env:ProgramData "ssh"
    if (-not (Test-Path $configDir)) {
        Write-Info ("Creating ssh configuration directory at {0}" -f $configDir)
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }

    $defaultConfig = Join-Path (Split-Path $exePath) "sshd_config_default"
    $configPath = Join-Path $configDir "sshd_config"
    if (-not (Test-Path $configPath) -and (Test-Path $defaultConfig)) {
        Write-Info "Seeding sshd_config from default template."
        Copy-Item -Path $defaultConfig -Destination $configPath -Force
    }

    $hostKey = Join-Path $configDir "ssh_host_ed25519_key"
    if (-not (Test-Path $hostKey)) {
        $sshKeygen = Join-Path (Split-Path $exePath) "ssh-keygen.exe"
        if (Test-Path $sshKeygen) {
            Write-Info "Generating host keys via ssh-keygen -A."
            & $sshKeygen -A | Out-Null
        }
    }

    Write-Info "Creating sshd service manually."
    & sc.exe create sshd binPath= $exePath start= auto DisplayName= "OpenSSH SSH Server" | Out-Null
    & sc.exe description sshd "OpenSSH SSH Server" | Out-Null
}

function Ensure-SshdRegistered {
    $desiredExePattern = [System.IO.Path]::Combine($env:SystemRoot, "System32\OpenSSH\sshd.exe")
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='sshd'" -ErrorAction SilentlyContinue

    if ($service -and $service.PathName) {
        $normalizedPath = $service.PathName.Trim('"')
        if ($normalizedPath -like "*$desiredExePattern*") {
            Write-Info "sshd service already registered against built-in OpenSSH."
            return
        }

        Write-Info "Removing existing sshd service registration."
        try {
            if ($service.State -ne "Stopped") {
                Stop-Service -Name "sshd" -Force -ErrorAction SilentlyContinue
            }
        }
        catch {
            Write-Info ("Failed to stop service 'sshd': {0}" -f $_.Exception.Message)
        }

        & sc.exe delete sshd | Out-Null
    }

    $installScript = Join-Path $env:SystemRoot "System32\OpenSSH\Install-SSHD.ps1"
    if (-not (Test-Path $installScript)) {
        Write-Info "System Install-SSHD.ps1 missing. Reinstalling OpenSSH optional features."
        foreach ($cap in @("OpenSSH.Client~~~~0.0.1.0", "OpenSSH.Server~~~~0.0.1.0")) {
            try {
                Remove-WindowsCapability -Online -Name $cap -ErrorAction SilentlyContinue | Out-Null
            }
            catch {
                Write-Info ("Failed to remove capability '{0}': {1}" -f $cap, $_.Exception.Message)
            }
            Write-Info ("Reinstalling Windows capability '{0}'" -f $cap)
            Add-WindowsCapability -Online -Name $cap -ErrorAction Stop | Out-Null
        }

        if (-not (Test-Path $installScript)) {
            $bundleInstaller = Join-Path $PSScriptRoot "OpenSSH-Win64\install-sshd.ps1"
            if (-not (Test-Path $bundleInstaller)) {
                Write-Info "Bundled installer missing; performing manual sshd service registration."
                Register-SshdServiceManual
                return
            }

            Write-Info "System Install-SSHD.ps1 still missing. Using bundled installer."
            & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $bundleInstaller
            return
        }
    }

    Write-Info "Registering sshd service using Install-SSHD.ps1"
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installScript
}

function Ensure-SshdService {
    $serviceName = "sshd"
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if (-not $service) {
        throw "Service '$serviceName' is missing. Ensure OpenSSH is installed correctly."
    }

    if ($service.StartType -ne "Automatic") {
        Write-Info "Setting service 'sshd' startup type to Automatic."
        Set-Service -Name $serviceName -StartupType Automatic -ErrorAction Stop
    }

    if ($service.Status -ne "Running") {
        Write-Info "Starting service 'sshd'."
        Start-Service -Name $serviceName -ErrorAction Stop
    }
    else {
        Write-Info "Service 'sshd' already running."
    }
}

function Ensure-VagrantKeys {
    $targetUser = "vagrant"
    $userProfile = Join-Path "C:\Users" $targetUser
    if (-not (Test-Path $userProfile)) {
        Write-Info ("User profile '{0}' not found; skipping Vagrant key provisioning." -f $userProfile)
        return
    }

    $sshDir = Join-Path $userProfile ".ssh"
    if (-not (Test-Path $sshDir)) {
        Write-Info ("Creating SSH directory at {0}" -f $sshDir)
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
    }

    $authorizedKeysPath = Join-Path $sshDir "authorized_keys"
    $keys = @(
        "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ== vagrant insecure public key",
        "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN1YdxBpNlzxDqfJyw/QKow1F+wvG9hXGoqiysfJOn5Y vagrant insecure public key"
    )

    $existing = @()
    if (Test-Path $authorizedKeysPath) {
        $existing = Get-Content -Path $authorizedKeysPath -ErrorAction SilentlyContinue
    }

    foreach ($key in $keys) {
        if ($existing -and ($existing | ForEach-Object { $_.Trim() }) -contains $key) {
            continue
        }

        if (-not (Test-Path $authorizedKeysPath)) {
            Set-Content -Path $authorizedKeysPath -Value $key -Encoding ascii
        }
        else {
            Add-Content -Path $authorizedKeysPath -Value $key -Encoding ascii
        }

        Write-Info ("Added key to {0}" -f $authorizedKeysPath)
    }

    try {
        & icacls $sshDir `
            /inheritance:r `
            /grant:r ("{0}:(OI)(CI)F" -f $targetUser) `
            /grant:r "Administrators:(OI)(CI)F" `
            /grant:r "SYSTEM:(OI)(CI)F" | Out-Null
    }
    catch {
        Write-Info ("Failed to adjust ACL for '{0}': {1}" -f $sshDir, $_.Exception.Message)
    }

    if (Test-Path $authorizedKeysPath) {
        try {
            & icacls $authorizedKeysPath `
                /inheritance:r `
                /grant:r ("{0}:F" -f $targetUser) `
                /grant:r "Administrators:F" `
                /grant:r "SYSTEM:F" | Out-Null
        }
        catch {
            Write-Info ("Failed to adjust ACL for '{0}': {1}" -f $authorizedKeysPath, $_.Exception.Message)
        }
    }
}

Ensure-OpenSshFeature
Ensure-SshdRegistered
Ensure-SshdService
Ensure-VagrantKeys
Write-Info "OpenSSH provisioning completed."
