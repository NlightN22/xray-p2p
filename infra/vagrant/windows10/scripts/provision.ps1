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
            Write-Info ("Failed to set Private profile for interface index {0}: {1}" -f $entry.InterfaceIndex, $_.Exception.Message)
        }
    }
}

function Disable-FirewallProfiles {
    $profiles = @("Domain", "Private", "Public")
    Write-Info ("Disabling Windows Firewall profiles: {0}" -f ($profiles -join ", "))
    foreach ($fwProfile in $profiles) {
        try {
            Set-NetFirewallProfile -Profile $fwProfile -Enabled False -ErrorAction Stop
            Write-Info "Firewall profile '$fwProfile' disabled."
        }
        catch {
            Write-Info ("Failed to disable firewall profile '{0}': {1}" -f $fwProfile, $_.Exception.Message)
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
            Write-Info ("Failed to remove existing IPv4 address {0} on '{1}': {2}" -f $entry.IPAddress, $InterfaceAlias, $_.Exception.Message)
        }
    }

    try {
        New-NetIPAddress -InterfaceAlias $InterfaceAlias -IPAddress $IPAddress -PrefixLength $PrefixLength -ErrorAction Stop | Out-Null
    }
    catch {
        Write-Info ("Failed to assign IP {0} to '{1}': {2}" -f $IPAddress, $InterfaceAlias, $_.Exception.Message)
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
        Write-Info ("SSH host key checking disabled for patterns: {0}" -f ($Patterns -join ", "))
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

Write-Info ("Provisioning role detected: {0}" -f $xp2pRole)

$hostOnlyAlias = if ($env:XP2P_HOSTONLY_ALIAS) { $env:XP2P_HOSTONLY_ALIAS } else { "Ethernet 2" }
$hostOnlyAddress = switch ($xp2pRole) {
    "server" { "10.0.10.10" }
    "client" { "10.0.10.20" }
}

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Go
Ensure-WiX
Disable-IdleSleepAndHibernate
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
Write-Info "Provisioning completed successfully."
