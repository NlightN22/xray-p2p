$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [System.Security.Principal.WindowsPrincipal]::new($currentIdentity)
if (-not $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "OpenSSH provisioning requires administrative privileges. Rerun this script from an elevated PowerShell session."
}

$Xp2pSshServiceName = "xp2p-sshd"
$Xp2pSshPort = 2222
$Xp2pSshConfigDir = Join-Path $env:ProgramData "xp2p-ssh"

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

function Ensure-Xp2pSshdConfig {
    $changed = $false
    $exePath = Join-Path $env:SystemRoot "System32\OpenSSH\sshd.exe"
    if (-not (Test-Path $exePath)) {
        throw "OpenSSH sshd.exe not found at $exePath"
    }

    if (-not (Test-Path $Xp2pSshConfigDir)) {
        Write-Info ("Creating xp2p ssh config directory at {0}" -f $Xp2pSshConfigDir)
        New-Item -ItemType Directory -Path $Xp2pSshConfigDir -Force | Out-Null
        $changed = $true
    }

    $sshKeygen = Join-Path (Split-Path $exePath) "ssh-keygen.exe"
    $ed25519Key = Join-Path $Xp2pSshConfigDir "ssh_host_ed25519_key"
    $rsaKey = Join-Path $Xp2pSshConfigDir "ssh_host_rsa_key"
    $hostKeys = @()

    if (-not (Test-Path $ed25519Key) -and (Test-Path $sshKeygen)) {
        Write-Info "Generating ed25519 host key."
        & $sshKeygen -t ed25519 -f $ed25519Key -N "" | Out-Null
        $changed = $true
    }
    if (Test-Path $ed25519Key) {
        $hostKeys += $ed25519Key
    }

    if (-not (Test-Path $rsaKey) -and (Test-Path $sshKeygen)) {
        Write-Info "Generating rsa host key."
        & $sshKeygen -t rsa -b 4096 -f $rsaKey -N "" | Out-Null
        $changed = $true
    }
    if (Test-Path $rsaKey) {
        $hostKeys += $rsaKey
    }

    if ($hostKeys.Count -eq 0) {
        Write-Info "No host keys available; xp2p-sshd may fail to start."
    }

    $authorizedKeys = "C:/Users/vagrant/.ssh/authorized_keys"
    $sftpPath = (Join-Path $env:SystemRoot "System32\OpenSSH\sftp-server.exe") -replace "\\", "/"
    $hostKeyLines = $hostKeys | ForEach-Object { ("HostKey {0}" -f ($_.ToString() -replace "\\", "/")) }

    $config = @(
        "Port $Xp2pSshPort",
        "Protocol 2"
    ) + $hostKeyLines + @(
        "AuthorizedKeysFile $authorizedKeys",
        "PasswordAuthentication no",
        "PubkeyAuthentication yes",
        "Subsystem sftp $sftpPath"
    )
    $configText = ($config -join "`r`n") + "`r`n"

    $configPath = Join-Path $Xp2pSshConfigDir "sshd_config"
    $existing = ""
    if (Test-Path $configPath) {
        $existing = Get-Content -Path $configPath -Raw -ErrorAction SilentlyContinue
    }

    if ($existing -ne $configText) {
        Set-Content -Path $configPath -Encoding ascii -Value $configText
        Write-Info ("Updated xp2p sshd_config at {0}" -f $configPath)
        $changed = $true
    }

    return $changed
}

function Ensure-Xp2pSshdService {
    $exePath = Join-Path $env:SystemRoot "System32\OpenSSH\sshd.exe"
    $configPath = Join-Path $Xp2pSshConfigDir "sshd_config"
    $logPath = Join-Path $Xp2pSshConfigDir "sshd.log"

    $service = Get-CimInstance -ClassName Win32_Service -Filter ("Name='{0}'" -f $Xp2pSshServiceName) -ErrorAction SilentlyContinue
    $binPath = "`"$exePath`" -f `"$configPath`" -E `"$logPath`""

    if (-not $service) {
        Write-Info ("Creating service '{0}'." -f $Xp2pSshServiceName)
        & sc.exe create $Xp2pSshServiceName binPath= $binPath start= auto DisplayName= "xp2p OpenSSH Server" | Out-Null
        & sc.exe description $Xp2pSshServiceName "xp2p OpenSSH Server" | Out-Null
    }

    try {
        Set-Service -Name $Xp2pSshServiceName -StartupType Automatic -ErrorAction Stop
    }
    catch {
        Write-Info ("Failed to set service '{0}' startup type: {1}" -f $Xp2pSshServiceName, $_.Exception.Message)
        return
    }

    try {
        Start-Service -Name $Xp2pSshServiceName -ErrorAction Stop
    }
    catch {
        Write-Info ("Failed to start service '{0}': {1}" -f $Xp2pSshServiceName, $_.Exception.Message)
    }
}

function Restart-Xp2pSshdService {
    $service = Get-Service -Name $Xp2pSshServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        Write-Info ("Service '{0}' not present; skipping restart." -f $Xp2pSshServiceName)
        return
    }

    try {
        Restart-Service -Name $Xp2pSshServiceName -Force -ErrorAction Stop
        Write-Info ("Service '{0}' restarted to apply changes." -f $Xp2pSshServiceName)
    }
    catch {
        Write-Info ("Failed to restart service '{0}': {1}" -f $Xp2pSshServiceName, $_.Exception.Message)
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
$keysChanged = Ensure-VagrantKeys
$defaultShellChanged = Ensure-DefaultOpenSshShell
$configChanged = Ensure-Xp2pSshdConfig
Ensure-Xp2pSshdService
if ($configChanged -or $keysChanged) {
    Restart-Xp2pSshdService
}
else {
    Write-Info ("Service '{0}' restart not required." -f $Xp2pSshServiceName)
}
Write-Info "OpenSSH provisioning completed."
