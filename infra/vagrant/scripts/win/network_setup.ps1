param(
    [Parameter(Mandatory = $true)]
    [string] $IPAddress
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
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

    $configuredAddress = Get-NetIPAddress -InterfaceAlias $InterfaceAlias -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.IPAddress -eq $IPAddress }
    if ($configuredAddress) {
        Write-Info "Host-only interface '$InterfaceAlias' successfully set to $IPAddress/$($configuredAddress.PrefixLength)."
    }
    else {
        Write-Info "Warning: host-only interface '$InterfaceAlias' did not report expected IP $IPAddress."
    }
}

function Set-PrivateNetworkProfile {
    param(
        [string] $AddressPrefixPattern = "10.62.10.",
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

function Disable-SshHostKeyChecking {
    param(
        [string] $TargetUser = "vagrant",
        [string[]] $Patterns = @("10.62.10.*")
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

$hostOnlyAlias = if ($env:XP2P_HOSTONLY_ALIAS) { $env:XP2P_HOSTONLY_ALIAS } else { "Ethernet 2" }
Set-HostOnlyAddress -InterfaceAlias $hostOnlyAlias -IPAddress $IPAddress
Set-PrivateNetworkProfile -AddressPrefixPattern "10.62.10."
Disable-FirewallProfiles
Disable-SshHostKeyChecking -Patterns @("10.62.10.*")
