$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

function Ensure-KeyPresent {
    param(
        [Parameter(Mandatory = $true)]
        [string] $KeyValue,

        [Parameter(Mandatory = $true)]
        [string] $AuthorizedKeysPath,

        [string] $Description = "key"
    )

    if ([string]::IsNullOrWhiteSpace($KeyValue)) {
        return
    }

    $normalized = $KeyValue.Trim()
    if ([string]::IsNullOrWhiteSpace($normalized)) {
        return
    }

    if (-not (Test-Path $AuthorizedKeysPath)) {
        Set-Content -Path $AuthorizedKeysPath -Value $normalized -Encoding ascii
        Write-Info ("Added {0} to {1}" -f $Description, $AuthorizedKeysPath)
        return
    }

    $existingKeys = Get-Content -Path $AuthorizedKeysPath -ErrorAction SilentlyContinue
    if ($existingKeys -and ($existingKeys | ForEach-Object { $_.Trim() }) -contains $normalized) {
        Write-Info ("{0} already present." -f $Description)
        return
    }

    Add-Content -Path $AuthorizedKeysPath -Value $normalized -Encoding ascii
    Write-Info ("Added {0} to {1}" -f $Description, $AuthorizedKeysPath)
}

function Ensure-OpenSsh {
    $capabilities = @(
        "OpenSSH.Client~~~~0.0.1.0",
        "OpenSSH.Server~~~~0.0.1.0"
    )

    foreach ($capability in $capabilities) {
        $current = Get-WindowsCapability -Online -Name $capability -ErrorAction SilentlyContinue
        if (-not $current -or $current.State -ne "Installed") {
            Write-Info ("Installing Windows capability '{0}'" -f $capability)
            Add-WindowsCapability -Online -Name $capability -ErrorAction Stop | Out-Null
        }
        else {
            Write-Info ("Windows capability '{0}' already installed." -f $capability)
        }
    }

    $serviceName = "sshd"
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($service) {
        try {
            Set-Service -Name $serviceName -StartupType Automatic -ErrorAction Stop
        }
        catch {
            Write-Info ("Failed to configure startup type for service '{0}': {1}" -f $serviceName, $_.Exception.Message)
        }

        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status -ne "Running") {
            try {
                Start-Service -Name $serviceName -ErrorAction Stop
                Write-Info "Service 'sshd' started."
            }
            catch {
                Write-Info ("Failed to start service '{0}': {1}" -f $serviceName, $_.Exception.Message)
            }
        }
        elseif ($service) {
            Write-Info "Service 'sshd' already running."
        }
    }
    else {
        Write-Info "Service 'sshd' not detected after capability installation attempt."
    }
}

function Ensure-SshdConfig {
    $configPath = "C:\ProgramData\ssh\sshd_config"
    if (-not (Test-Path $configPath)) {
        Write-Info "sshd_config not found; skipping configuration update."
        return
    }

    try {
        $existing = Get-Content -Path $configPath -ErrorAction Stop
    }
    catch {
        Write-Info ("Unable to read {0}: {1}" -f $configPath, $_.Exception.Message)
        return
    }

    $marker = "# xp2p-sshd-config"
    if ($existing | Where-Object { $_ -eq $marker }) {
        Write-Info "sshd_config already contains xp2p overrides."
        return
    }

    $block = @()
    if ($existing.Length -gt 0) {
        $block += ""
    }
    $block += $marker
    $block += "AuthorizedKeysFile __PROGRAMDATA__/ssh/authorized_keys %h/.ssh/authorized_keys"
    $block += "PubkeyAuthentication yes"
    $block += "PasswordAuthentication yes"

    try {
        $block | Add-Content -Path $configPath -Encoding ascii
        Write-Info ("Appended xp2p sshd overrides to {0}" -f $configPath)
    }
    catch {
        Write-Info ("Failed to update {0}: {1}" -f $configPath, $_.Exception.Message)
        return
    }

    try {
        Restart-Service -Name "sshd" -Force -ErrorAction Stop
        Write-Info "Service 'sshd' restarted to apply configuration."
    }
    catch {
        Write-Info ("Failed to restart service 'sshd': {0}" -f $_.Exception.Message)
    }
}

function Ensure-VagrantKeys {
    param(
        [string] $TargetUser = "vagrant"
    )

    $userProfile = Join-Path "C:\Users" $TargetUser
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
    $insecureKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ== vagrant insecure public key"

    Ensure-KeyPresent -KeyValue $insecureKey -AuthorizedKeysPath $authorizedKeysPath -Description "Vagrant insecure public key"

    $machineId = $env:XP2P_MACHINE_ID
    $sharedRoot = if ($env:XP2P_SYNC_ROOT) { $env:XP2P_SYNC_ROOT } else { "C:\xp2p" }
    if (-not [string]::IsNullOrWhiteSpace($machineId)) {
        $machineKeyPath = Join-Path $sharedRoot (Join-Path ".vagrant\machines" (Join-Path $machineId "virtualbox\private_key"))
        if (Test-Path $machineKeyPath) {
            try {
                $machinePublicKey = & ssh-keygen.exe -y -f $machineKeyPath 2>$null
                if ($machinePublicKey) {
                    Ensure-KeyPresent -KeyValue $machinePublicKey -AuthorizedKeysPath $authorizedKeysPath -Description ("machine-specific key from {0}" -f $machineKeyPath)
                }
            }
            catch {
                Write-Info ("Failed to derive public key from '{0}': {1}" -f $machineKeyPath, $_.Exception.Message)
            }
        }
    }

    try {
        $directoryGrants = @(
            "{0}:(OI)(CI)F" -f $TargetUser,
            "Administrators:(OI)(CI)F",
            "SYSTEM:(OI)(CI)F",
            '"NT SERVICE\SSHD":(OI)(CI)R'
        )
        $dirArgs = @("/inheritance:r")
        foreach ($grant in $directoryGrants) {
            $dirArgs += @("/grant:r", $grant)
        }
        & icacls $sshDir @dirArgs | Out-Null

        $fileGrants = @(
            "{0}:F" -f $TargetUser,
            "Administrators:F",
            "SYSTEM:F",
            '"NT SERVICE\SSHD":R'
        )
        $fileArgs = @("/inheritance:r")
        foreach ($grant in $fileGrants) {
            $fileArgs += @("/grant:r", $grant)
        }
        & icacls $authorizedKeysPath @fileArgs | Out-Null
    }
    catch {
        Write-Info ("Failed to adjust ACL for '{0}': {1}" -f $sshDir, $_.Exception.Message)
    }
}

Ensure-OpenSsh
Ensure-SshdConfig
Ensure-VagrantKeys -TargetUser "vagrant"
Write-Info "OpenSSH provisioning completed."
