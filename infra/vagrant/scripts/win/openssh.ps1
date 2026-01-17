$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [System.Security.Principal.WindowsPrincipal]::new($currentIdentity)
if (-not $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "OpenSSH provisioning requires administrative privileges. Rerun this script from an elevated PowerShell session."
}

$SshdServiceName = "sshd"
$SshdPort = 2222
$SshConfigDir = Join-Path $env:ProgramData "ssh"
$OpenSshBinDir = Join-Path $env:SystemRoot "System32\OpenSSH"
$script:OpenSshRestartRequired = $false

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

function Ensure-OpenSshCapability {
    $caps = @(
        "OpenSSH.Client~~~~0.0.1.0",
        "OpenSSH.Server~~~~0.0.1.0"
    )

    foreach ($cap in $caps) {
        $state = (Get-WindowsCapability -Online -Name $cap -ErrorAction SilentlyContinue).State
        if ($state -ne "Installed") {
            Write-Info ("Installing Windows capability '{0}'." -f $cap)
            $result = Add-WindowsCapability -Online -Name $cap
            if ($result -and $result.RestartNeeded) {
                $script:OpenSshRestartRequired = $true
            }
        }
        else {
            Write-Info ("Windows capability '{0}' already installed." -f $cap)
        }
    }
}

function Ensure-AclRules {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Path,

        [Parameter(Mandatory = $true)]
        [array] $ExpectedRules,

        [Parameter(Mandatory = $true)]
        [string[]] $IcaclsArguments
    )

    if (-not (Test-Path $Path)) {
        return $false
    }

    $acl = Get-Acl -Path $Path -ErrorAction SilentlyContinue
    if (-not $acl) {
        return $false
    }

    $allPresent = $true
    foreach ($rule in $ExpectedRules) {
        $matched = $false
        foreach ($entry in $acl.Access) {
            $identityValue = $entry.IdentityReference.Value
            if (-not (& $rule.Match $identityValue)) {
                continue
            }

            if ($entry.AccessControlType -ne [System.Security.AccessControl.AccessControlType]::Allow) {
                continue
            }

            if (($entry.FileSystemRights -band $rule.Rights) -ne $rule.Rights) {
                continue
            }

            if (($entry.InheritanceFlags -band $rule.Inheritance) -ne $rule.Inheritance) {
                continue
            }

            if ($entry.PropagationFlags -ne $rule.Propagation) {
                continue
            }

            $matched = $true
            break
        }

        if (-not $matched) {
            $allPresent = $false
            break
        }
    }

    if ($allPresent) {
        return $false
    }

    & icacls @IcaclsArguments | Out-Null
    return $true
}

function Repair-HostKeyPermissions {
    param([string] $KeyPath)

    if (-not (Test-Path $KeyPath)) {
        return
    }

    & icacls $KeyPath /inheritance:r | Out-Null
    & icacls $KeyPath /grant:r "SYSTEM:F" "Administrators:F" | Out-Null

    $pubPath = "$KeyPath.pub"
    if (Test-Path $pubPath) {
        & icacls $pubPath /inheritance:r | Out-Null
        & icacls $pubPath /grant:r "SYSTEM:F" "Administrators:F" "Everyone:R" | Out-Null
    }
}

function Ensure-SshdConfig {
    $changed = $false
    $exePath = Join-Path $OpenSshBinDir "sshd.exe"
    if (-not (Test-Path $exePath)) {
        throw "OpenSSH sshd.exe not found at $exePath"
    }

    if (-not (Test-Path $SshConfigDir)) {
        Write-Info ("Creating ssh config directory at {0}" -f $SshConfigDir)
        New-Item -ItemType Directory -Path $SshConfigDir -Force | Out-Null
        $changed = $true
    }

    $sshKeygen = Join-Path $OpenSshBinDir "ssh-keygen.exe"
    $ed25519Key = Join-Path $SshConfigDir "ssh_host_ed25519_key"
    $rsaKey = Join-Path $SshConfigDir "ssh_host_rsa_key"
    $hostKeys = @()

    foreach ($candidate in @($ed25519Key, $rsaKey)) {
        if (Test-Path $candidate) {
            Write-Info ("Found host key: {0}" -f $candidate)
            $hostKeys += $candidate
        }
    }

    if ($hostKeys.Count -eq 0) {
        Write-Info "No host keys found; generating system host keys."
        if (-not (Test-Path $sshKeygen)) {
            throw "ssh-keygen.exe not found at $sshKeygen"
        }
        & $sshKeygen -A | Out-Null
        $changed = $true

        foreach ($candidate in @($ed25519Key, $rsaKey)) {
            if (Test-Path $candidate) {
                Write-Info ("Generated host key: {0}" -f $candidate)
                $hostKeys += $candidate
            }
        }
    }

    foreach ($hostKey in $hostKeys) {
        Write-Info ("Repairing host key permissions: {0}" -f $hostKey)
        Repair-HostKeyPermissions -KeyPath $hostKey
    }

    if ($hostKeys.Count -eq 0) {
        throw "No host keys available after generation attempts."
    }

    $defaultConfig = Join-Path $OpenSshBinDir "sshd_config_default"
    $configLines = @()
    if (Test-Path $defaultConfig) {
        $configLines = Get-Content -Path $defaultConfig -ErrorAction SilentlyContinue
    }

    $hasSubsystem = $false
    foreach ($line in $configLines) {
        if ($line -match '^\s*Subsystem\s+sftp\b') {
            $hasSubsystem = $true
            break
        }
    }

    $override = @(
        "Port $SshdPort",
        "PermitTTY yes",
        "StrictModes no",
        "PubkeyAcceptedKeyTypes +ssh-rsa",
        "HostKeyAlgorithms +ssh-rsa",
        "AuthorizedKeysFile .ssh/authorized_keys",
        "PasswordAuthentication no",
        "PubkeyAuthentication yes"
    )
    if (-not $hasSubsystem) {
        $override += "Subsystem sftp sftp-server.exe"
    }

    $finalLines = @()
    $inserted = $false
    foreach ($line in $configLines) {
        if (-not $inserted -and $line -match '^\s*Match\s+') {
            $finalLines += ""
            $finalLines += $override
            $finalLines += ""
            $inserted = $true
        }
        $finalLines += $line
    }
    if (-not $inserted) {
        $finalLines += ""
        $finalLines += $override
        $finalLines += ""
    }

    $configText = ($finalLines -join "`r`n") + "`r`n"
    $configPath = Join-Path $SshConfigDir "sshd_config"
    $existing = ""
    if (Test-Path $configPath) {
        $existing = Get-Content -Path $configPath -Raw -ErrorAction SilentlyContinue
    }

    if ($existing -ne $configText) {
        Set-Content -Path $configPath -Encoding ascii -Value $configText
        Write-Info ("Updated sshd_config at {0}" -f $configPath)
        $changed = $true
    }

    $testOutput = & $exePath -t -f $configPath 2>&1
    $testExit = $LASTEXITCODE
    if ($testOutput) {
        Write-Info "sshd.exe -t output:"
        $testOutput | ForEach-Object { Write-Host $_ }
    }
    if ($testExit -ne 0) {
        throw "sshd.exe -t failed with exit code $testExit"
    }

    return $changed
}

function Ensure-SshdService {
    $service = Get-Service -Name $SshdServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        throw "OpenSSH service '$SshdServiceName' not found; install OpenSSH first."
    }

    try {
        Set-Service -Name $SshdServiceName -StartupType Automatic -ErrorAction Stop
    }
    catch {
        Write-Info ("Failed to set service '{0}' startup type: {1}" -f $SshdServiceName, $_.Exception.Message)
        return
    }
}

function Ensure-SshFirewall {
    $ruleName = "xp2p-sshd"
    $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    if ($existing) {
        return
    }
    try {
        New-NetFirewallRule -DisplayName $ruleName -Direction Inbound -Action Allow -Protocol TCP -LocalPort $SshdPort | Out-Null
        Write-Info ("Firewall rule '{0}' added for port {1}." -f $ruleName, $SshdPort)
    }
    catch {
        Write-Info ("Failed to add firewall rule '{0}': {1}" -f $ruleName, $_.Exception.Message)
    }
}

function Restart-SshdService {
    param(
        [bool] $ForceRestart = $false
    )

    $service = Get-Service -Name $SshdServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Info ("Service '{0}' not present; skipping restart." -f $SshdServiceName)
        return
    }

    try {
        if ($ForceRestart -and $service.Status -ne "Stopped") {
            Stop-Service -Name $SshdServiceName -Force -ErrorAction Stop
            Start-Sleep -Seconds 2
        }
    }
    catch {
        Write-Info ("Failed to stop service '{0}': {1}" -f $SshdServiceName, $_.Exception.Message)
    }

    $service = Get-Service -Name $SshdServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Info ("Service '{0}' not present after stop; skipping start." -f $SshdServiceName)
        return
    }

    if ($service.Status -ne "Running") {
        $started = $false
        for ($attempt = 1; $attempt -le 3; $attempt += 1) {
            try {
                Start-Service -Name $SshdServiceName -ErrorAction Stop
                Start-Sleep -Seconds 2
                $service = Get-Service -Name $SshdServiceName -ErrorAction SilentlyContinue
                if ($service -and $service.Status -eq "Running") {
                    $started = $true
                    break
                }
            }
            catch {
                Write-Info ("Start attempt {0} failed: {1}" -f $attempt, $_.Exception.Message)
                Start-Sleep -Seconds 2
            }
        }

        if ($started) {
            Write-Info ("Service '{0}' started." -f $SshdServiceName)
            return
        }

        Write-Info ("Failed to start service '{0}'." -f $SshdServiceName)
        Write-Info "Service status (sc query):"
        & sc.exe query $SshdServiceName | ForEach-Object { Write-Host $_ }
        return
    }

    if ($ForceRestart) {
        Write-Info ("Service '{0}' restarted to apply changes." -f $SshdServiceName)
    }
    else {
        Write-Info ("Service '{0}' already running." -f $SshdServiceName)
    }
}

function Ensure-VagrantKeys {
    $targetUser = "vagrant"
    $userProfile = Join-Path "C:\Users" $targetUser
    if (-not (Test-Path $userProfile)) {
        Write-Info ("User profile '{0}' not found; skipping Vagrant key provisioning." -f $userProfile)
        return $false
    }

    $changes = $false
    $sshDir = Join-Path $userProfile ".ssh"
    if (-not (Test-Path $sshDir)) {
        Write-Info ("Creating SSH directory at {0}" -f $sshDir)
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
        $changes = $true
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
        $changes = $true
    }

    $inheritAll = [System.Security.AccessControl.InheritanceFlags]::ContainerInherit -bor [System.Security.AccessControl.InheritanceFlags]::ObjectInherit
    $inheritNone = [System.Security.AccessControl.InheritanceFlags]::None
    $propagationNone = [System.Security.AccessControl.PropagationFlags]::None
    $fullControl = [System.Security.AccessControl.FileSystemRights]::FullControl

    $matchUser = {
        param($id)
        if (-not $id) { return $false }
        $id.Equals($targetUser, [System.StringComparison]::OrdinalIgnoreCase) -or
            $id.EndsWith("\$targetUser", [System.StringComparison]::OrdinalIgnoreCase)
    }
    $matchAdmins = {
        param($id)
        if (-not $id) { return $false }
        $id.Equals("BUILTIN\Administrators", [System.StringComparison]::OrdinalIgnoreCase) -or
            $id.EndsWith("\Administrators", [System.StringComparison]::OrdinalIgnoreCase)
    }
    $matchSystem = {
        param($id)
        if (-not $id) { return $false }
        $id.Equals("NT AUTHORITY\SYSTEM", [System.StringComparison]::OrdinalIgnoreCase)
    }

    $dirRules = @(
        @{ Match = $matchUser; Rights = $fullControl; Inheritance = $inheritAll; Propagation = $propagationNone },
        @{ Match = $matchAdmins; Rights = $fullControl; Inheritance = $inheritAll; Propagation = $propagationNone },
        @{ Match = $matchSystem; Rights = $fullControl; Inheritance = $inheritAll; Propagation = $propagationNone }
    )
    $dirIcaclsArgs = @(
        $sshDir,
        "/inheritance:r",
        "/grant:r", ("{0}:(OI)(CI)F" -f $targetUser),
        "/grant:r", "Administrators:(OI)(CI)F",
        "/grant:r", "SYSTEM:(OI)(CI)F"
    )
    if (Ensure-AclRules -Path $sshDir -ExpectedRules $dirRules -IcaclsArguments $dirIcaclsArgs) {
        $changes = $true
    }

    if (Test-Path $authorizedKeysPath) {
        $fileRules = @(
            @{ Match = $matchUser; Rights = $fullControl; Inheritance = $inheritNone; Propagation = $propagationNone },
            @{ Match = $matchAdmins; Rights = $fullControl; Inheritance = $inheritNone; Propagation = $propagationNone },
            @{ Match = $matchSystem; Rights = $fullControl; Inheritance = $inheritNone; Propagation = $propagationNone }
        )
        $fileIcaclsArgs = @(
            $authorizedKeysPath,
            "/inheritance:r",
            "/grant:r", ("{0}:F" -f $targetUser),
            "/grant:r", "Administrators:F",
            "/grant:r", "SYSTEM:F"
        )
        if (Ensure-AclRules -Path $authorizedKeysPath -ExpectedRules $fileRules -IcaclsArguments $fileIcaclsArgs) {
            $changes = $true
        }
    }

    $adminKeysPath = Join-Path $env:ProgramData "ssh\administrators_authorized_keys"
    $adminExisting = @()
    if (Test-Path $adminKeysPath) {
        $adminExisting = Get-Content -Path $adminKeysPath -ErrorAction SilentlyContinue
    }

    foreach ($key in $keys) {
        if ($adminExisting -and ($adminExisting | ForEach-Object { $_.Trim() }) -contains $key) {
            continue
        }

        if (-not (Test-Path $adminKeysPath)) {
            Set-Content -Path $adminKeysPath -Value $key -Encoding ascii
        }
        else {
            Add-Content -Path $adminKeysPath -Value $key -Encoding ascii
        }
        $changes = $true
    }

    if (Test-Path $adminKeysPath) {
        & icacls $adminKeysPath /inheritance:r | Out-Null
        & icacls $adminKeysPath /grant:r "Administrators:F" "SYSTEM:F" | Out-Null
    }

    return $changes
}

function Ensure-DefaultOpenSshShell {
    param(
        [string] $ShellPath = "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
    )

    $regPath = "HKLM:\SOFTWARE\OpenSSH"
    $name = "DefaultShell"
    $optionName = "DefaultShellCommandOption"

    $current = $null
    $currentOption = $null
    try {
        $props = Get-ItemProperty -Path $regPath -ErrorAction Stop
        $current = $props.$name
        $currentOption = $props.$optionName
    }
    catch {
        # Property missing; will create it below.
    }

    $desiredOption = "-Command"
    if ($ShellPath -match "cmd\.exe$") {
        $desiredOption = "/c"
    }

    if ($current -and ($current.Trim()) -eq $ShellPath -and $currentOption -and ($currentOption.Trim()) -eq $desiredOption) {
        Write-Info ("Default OpenSSH shell already set to '{0}' with option '{1}'." -f $ShellPath, $desiredOption)
        return $false
    }

    Write-Info ("Setting OpenSSH default shell to '{0}' with option '{1}'." -f $ShellPath, $desiredOption)
    if (-not (Test-Path $regPath)) {
        New-Item -Path $regPath -Force | Out-Null
    }

    New-ItemProperty -Path $regPath -Name $name -Value $ShellPath -PropertyType String -Force | Out-Null
    New-ItemProperty -Path $regPath -Name $optionName -Value $desiredOption -PropertyType String -Force | Out-Null
    return $true
}

Ensure-OpenSshCapability
if ($script:OpenSshRestartRequired) {
    Write-Info "OpenSSH capability install requires a reboot; re-run provisioning after restart."
    exit 0
}
$keysChanged = Ensure-VagrantKeys
$defaultShellChanged = Ensure-DefaultOpenSshShell
$configChanged = Ensure-SshdConfig
Ensure-SshdService
Ensure-SshFirewall
Restart-SshdService -ForceRestart:($configChanged -or $keysChanged)
Write-Info "OpenSSH provisioning completed."
