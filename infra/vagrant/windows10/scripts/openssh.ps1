$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [System.Security.Principal.WindowsPrincipal]::new($currentIdentity)
if (-not $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "OpenSSH provisioning requires administrative privileges. Rerun this script from an elevated PowerShell session."
}

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

function Restart-SshdService {
    $serviceName = "sshd"
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Info "Service 'sshd' not present; skipping restart."
        return
    }

    try {
        Restart-Service -Name $serviceName -Force -ErrorAction Stop
        Write-Info "Service 'sshd' restarted to apply changes."
    }
    catch {
        Write-Info ("Failed to restart service 'sshd': {0}" -f $_.Exception.Message)
        try {
            Start-Service -Name $serviceName -ErrorAction Stop
            Write-Info "Service 'sshd' started."
        }
        catch {
            Write-Info ("Failed to start service 'sshd': {0}" -f $_.Exception.Message)
        }
    }
}

function Ensure-SshdConfigDefaults {
    $configPath = Join-Path $env:ProgramData "ssh\sshd_config"
    if (-not (Test-Path $configPath)) {
        Write-Info ("sshd_config not found at {0}; skipping config cleanup." -f $configPath)
        return $false
    }

    $lines = Get-Content -Path $configPath -ErrorAction Stop
    if (-not $lines) {
        return $false
    }

    $changed = $false
    $matchPattern = '^\s*Match\s+Group\s+administrators\b'
    $adminKeyPattern = '^\s*AuthorizedKeysFile\s+__PROGRAMDATA__/ssh/administrators_authorized_keys\b'

    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]

        if ($line -notmatch '^\s*#' -and $line -match $matchPattern) {
            $lines[$i] = "# Match Group administrators"
            $changed = $true
            continue
        }

        if ($line -notmatch '^\s*#' -and $line -match $adminKeyPattern) {
            $lines[$i] = "# AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys"
            $changed = $true
        }
    }

    if ($changed) {
        Set-Content -Path $configPath -Encoding ascii -Value $lines
        Write-Info ("Commented administrative override entries in {0}" -f $configPath)
    }

    return $changed
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

    return $changes
}

function Ensure-DefaultOpenSshShell {
    param(
        [string] $ShellPath = "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
    )

    $regPath = "HKLM:\SOFTWARE\OpenSSH"
    $name = "DefaultShell"

    $current = $null
    try {
        $current = (Get-ItemProperty -Path $regPath -Name $name -ErrorAction Stop).$name
    }
    catch {
        # Property missing; will create it below.
    }

    if ($current -and ($current.Trim()) -eq $ShellPath) {
        Write-Info ("Default OpenSSH shell already set to '{0}'." -f $ShellPath)
        return $false
    }

    Write-Info ("Setting OpenSSH default shell to '{0}'." -f $ShellPath)
    if (-not (Test-Path $regPath)) {
        New-Item -Path $regPath -Force | Out-Null
    }

    New-ItemProperty -Path $regPath -Name $name -Value $ShellPath -PropertyType String -Force | Out-Null
    return $true
}

Ensure-OpenSshFeature
Ensure-SshdRegistered
$configChanged = Ensure-SshdConfigDefaults
$keysChanged = Ensure-VagrantKeys
$defaultShellChanged = Ensure-DefaultOpenSshShell
Ensure-SshdService
if ($configChanged -or $keysChanged) {
    Restart-SshdService
}
else {
    Write-Info "Service 'sshd' restart not required."
}
Write-Info "OpenSSH provisioning completed."
