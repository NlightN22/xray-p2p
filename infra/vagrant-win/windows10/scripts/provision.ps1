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

function Set-PrivateNetworkProfile {
    param(
        [string] $AddressPrefixPattern = "10.0.10.",
        [int] $TimeoutSeconds = 60
    )

    Write-Info "Ensuring network interfaces matching '$AddressPrefixPattern*' use Private profile..."
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $interfaces = @()

    while ((Get-Date) -lt $deadline -and -not $interfaces) {
        $interfaces = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
            Where-Object { $_.IPAddress -like "$AddressPrefixPattern*" }
        if (-not $interfaces) {
            Start-Sleep -Seconds 2
        }
    }

    if (-not $interfaces) {
        Write-Info "No interfaces detected for prefix '$AddressPrefixPattern'; skipping profile adjustment."
        return
    }

    $processed = @{}
    foreach ($entry in $interfaces) {
        if ($processed.ContainsKey($entry.InterfaceIndex)) {
            continue
        }
        $processed[$entry.InterfaceIndex] = $true
        try {
            Set-NetConnectionProfile -InterfaceIndex $entry.InterfaceIndex -NetworkCategory Private -ErrorAction Stop
            Write-Info "Interface index $($entry.InterfaceIndex) set to Private."
        }
        catch {
            Write-Info "Failed to set Private profile for interface index $($entry.InterfaceIndex): $($_.Exception.Message)"
        }
    }
}

function Disable-FirewallProfiles {
    $profiles = @("Domain", "Private", "Public")
    Write-Info "Disabling Windows Firewall profiles: $($profiles -join ', ')"
    foreach ($fwProfile in $profiles) {
        try {
            Set-NetFirewallProfile -Profile $fwProfile -Enabled False -ErrorAction Stop
            Write-Info "Firewall profile '$fwProfile' disabled."
        }
        catch {
            Write-Info "Failed to disable firewall profile '$fwProfile': $($_.Exception.Message)"
        }
    }
}

function Set-HostOnlyAddress {
    param(
        [Parameter(Mandatory = $true)]
        [string] $InterfaceAlias,

        [Parameter(Mandatory = $true)]
        [string] $IPAddress,

        [int] $PrefixLength = 24
    )

    Write-Info "Configuring host-only interface '$InterfaceAlias' with IP $IPAddress/$PrefixLength"
    $existing = Get-NetIPAddress -InterfaceAlias $InterfaceAlias -AddressFamily IPv4 -ErrorAction SilentlyContinue
    foreach ($entry in $existing) {
        try {
            Remove-NetIPAddress -InputObject $entry -Confirm:$false -ErrorAction Stop
        }
        catch {
            Write-Info "Failed to remove existing IPv4 address $($entry.IPAddress) on '$InterfaceAlias': $($_.Exception.Message)"
        }
    }

    try {
        New-NetIPAddress -InterfaceAlias $InterfaceAlias -IPAddress $IPAddress -PrefixLength $PrefixLength -ErrorAction Stop | Out-Null
    }
    catch {
        Write-Info "Failed to assign IP $IPAddress to '$InterfaceAlias': $($_.Exception.Message)"
        throw
    }
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
        Write-Info "Installing Chocolatey package '$Package' (version: $Version)"
        choco $installArgs | Write-Host
    }
    else {
        Write-Info "Chocolatey package '$Package' already installed."
    }
}

function Ensure-OpenSsh {
    $capabilities = @(
        "OpenSSH.Client~~~~0.0.1.0",
        "OpenSSH.Server~~~~0.0.1.0"
    )

    foreach ($capability in $capabilities) {
        $current = Get-WindowsCapability -Online -Name $capability -ErrorAction SilentlyContinue
        if (-not $current -or $current.State -ne "Installed") {
            Write-Info "Installing Windows capability '$capability'"
            Add-WindowsCapability -Online -Name $capability -ErrorAction Stop | Out-Null
        }
        else {
            Write-Info "Windows capability '$capability' already installed."
        }
    }

    $serviceName = "sshd"
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($service) {
        try {
            Set-Service -Name $serviceName -StartupType Automatic -ErrorAction Stop
        }
        catch {
            Write-Info "Failed to configure startup type for service '$serviceName': $($_.Exception.Message)"
        }

        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status -ne "Running") {
            try {
                Start-Service -Name $serviceName -ErrorAction Stop
                Write-Info "Service '$serviceName' started."
            }
            catch {
                Write-Info "Failed to start service '$serviceName': $($_.Exception.Message)"
            }
        }
        elseif ($service) {
            Write-Info "Service '$serviceName' already running."
        }
    }
    else {
        Write-Info "Service '$serviceName' not detected after capability installation attempt."
    }
}

function Ensure-VagrantKeys {
    param(
        [string] $TargetUser = "vagrant"
    )

    $userProfile = Join-Path "C:\Users" $TargetUser
    if (-not (Test-Path $userProfile)) {
        Write-Info "User profile '$userProfile' not found; skipping Vagrant key provisioning."
        return
    }

    $sshDir = Join-Path $userProfile ".ssh"
    if (-not (Test-Path $sshDir)) {
        Write-Info "Creating SSH directory at $sshDir"
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
    }

    $authorizedKeysPath = Join-Path $sshDir "authorized_keys"
    $insecureKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ== vagrant insecure public key"

    if (-not (Test-Path $authorizedKeysPath)) {
        Write-Info "Creating Vagrant authorized_keys file."
        Set-Content -Path $authorizedKeysPath -Value $insecureKey -Encoding ascii
    }
    else {
        $existingKeys = Get-Content -Path $authorizedKeysPath -ErrorAction SilentlyContinue
        if ($existingKeys -and ($existingKeys | ForEach-Object { $_.Trim() }) -contains $insecureKey) {
            Write-Info "Vagrant insecure public key already present."
        }
        else {
            Write-Info "Appending Vagrant insecure public key."
            Add-Content -Path $authorizedKeysPath -Value $insecureKey -Encoding ascii
        }
    }

    try {
        $targetRule = "{0}:(OI)(CI)F" -f $TargetUser
        & icacls $sshDir /inheritance:r /grant:r $targetRule /grant:r "Administrators:(OI)(CI)F" | Out-Null
    }
    catch {
        Write-Info "Failed to adjust ACL for '$sshDir': $($_.Exception.Message)"
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
    Write-Info "Go toolchain ready: $goVersionOutput"
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

    $smokeHost = "127.0.0.1"
    $smokePort = 62022

    Write-Info "Starting xp2p diagnostics service on port $smokePort"
    $serverProcess = Start-Process -FilePath $xp2pExe.Source `
        -ArgumentList "--server-port", $smokePort `
        -PassThru `
        -WindowStyle Hidden `
        -RedirectStandardOutput "C:\tools\xp2p\diagnostics.log" `
        -RedirectStandardError "C:\tools\xp2p\diagnostics.err"

    try {
        if (-not (Wait-TcpPort -TargetHost $smokeHost -Port $smokePort -TimeoutSeconds 20)) {
            throw "xp2p diagnostics service failed to start on port $smokePort within timeout."
        }

        Write-Info "Running smoke test: xp2p ping $smokeHost --port $smokePort"
        & $xp2pExe.Source ping $smokeHost --port $smokePort | Write-Host
    }
    finally {
        if ($serverProcess -and -not $serverProcess.HasExited) {
            Write-Info "Stopping xp2p diagnostics service"
            Stop-Process -Id $serverProcess.Id -Force
        }
    }
}

function Disable-SshHostKeyChecking {
    param(
        [string] $TargetUser = "vagrant",
        [string[]] $Patterns = @("10.0.10.*")
    )

    if (-not $Patterns -or $Patterns.Count -eq 0) {
        return
    }

    $userProfile = Join-Path "C:\Users" $TargetUser
    if (-not (Test-Path $userProfile)) {
        Write-Info "User profile '$userProfile' missing; skipping SSH host key policy update."
        return
    }

    $sshDir = Join-Path $userProfile ".ssh"
    if (-not (Test-Path $sshDir)) {
        Write-Info "Creating SSH directory at $sshDir"
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
    }

    $configPath = Join-Path $sshDir "config"
    if (-not (Test-Path $configPath)) {
        New-Item -ItemType File -Path $configPath -Force | Out-Null
    }

    $existing = Get-Content $configPath -ErrorAction SilentlyContinue
    $marker = "# xp2p-disable-host-key-checking"
    if ($existing -and ($existing | Where-Object { $_ -eq $marker })) {
        Write-Info "SSH host key checking already disabled for target patterns."
        return
    }

    $block = @()
    if ($existing -and $existing.Count -gt 0) {
        $block += ""
    }
    $block += $marker
    foreach ($pattern in $Patterns) {
        $trimmed = $pattern.Trim()
        if ([string]::IsNullOrWhiteSpace($trimmed)) {
            continue
        }
        $block += "Host $trimmed"
        $block += "    StrictHostKeyChecking no"
        $block += "    UserKnownHostsFile NUL"
        $block += "    CheckHostIP no"
        $block += ""
    }

    if ($block.Count -gt 0) {
        $block | Add-Content -Path $configPath -Encoding ascii
        Write-Info "SSH host key checking disabled for patterns: $($Patterns -join ', ')"
    }
}

$xp2pRole = $env:XP2P_ROLE
if ([string]::IsNullOrWhiteSpace($xp2pRole)) {
    $xp2pRole = "server"
}
else {
    $xp2pRole = $xp2pRole.ToLowerInvariant()
}

if ($xp2pRole -notin @("server", "client")) {
    Write-Info "Unknown role '$xp2pRole'. Falling back to 'server'."
    $xp2pRole = "server"
}

Write-Info "Provisioning role detected: $xp2pRole"

$hostOnlyAlias = if ($env:XP2P_HOSTONLY_ALIAS) { $env:XP2P_HOSTONLY_ALIAS } else { "Ethernet 2" }
$hostOnlyAddress = switch ($xp2pRole) {
    "server" { "10.0.10.10" }
    "client" { "10.0.10.20" }
}

Ensure-IsElevated
Ensure-Chocolatey
Ensure-OpenSsh
Ensure-VagrantKeys -TargetUser "vagrant"
Ensure-Go
Build-Xp2p
Set-HostOnlyAddress -InterfaceAlias $hostOnlyAlias -IPAddress $hostOnlyAddress
$configuredAddress = Get-NetIPAddress -InterfaceAlias $hostOnlyAlias -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -eq $hostOnlyAddress }
if ($configuredAddress) {
    Write-Info "Host-only interface '$hostOnlyAlias' successfully set to $hostOnlyAddress/$($configuredAddress.PrefixLength)."
}
else {
    Write-Info "Warning: host-only interface '$hostOnlyAlias' did not report expected IP $hostOnlyAddress."
}
Set-PrivateNetworkProfile -AddressPrefixPattern "10.0.10."
Disable-FirewallProfiles
Disable-SshHostKeyChecking -Patterns @("10.0.10.*")
Run-SmokeTest -Skip:$false
Write-Info "Provisioning completed successfully."
