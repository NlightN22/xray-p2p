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

function Ensure-Xp2pFirewall {
    param(
        [Parameter(Mandatory = $true)]
        [int] $Port,

        [Parameter(Mandatory = $true)]
        [string] $Role
    )

    $protocols = @("TCP", "UDP")
    foreach ($proto in $protocols) {
        $ruleName = "xp2p-$Role-$proto-$Port"
        if (Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue) {
            Write-Info "Firewall rule '$ruleName' already present."
            continue
        }

        Write-Info "Creating firewall rule '$ruleName' for $proto port $Port (Private profile)."
        New-NetFirewallRule -DisplayName $ruleName -Name $ruleName `
            -Direction Inbound -Action Allow -Protocol $proto -LocalPort $Port -Profile Private | Out-Null
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

    try {
        Set-Service -Name winrm -StartupType Automatic
        if ((Get-Service -Name winrm).Status -ne 'Running') {
            Start-Service -Name winrm
        }
    }
    catch {
        Write-Info "Failed to start WinRM service: $($_.Exception.Message)"
    }

    try {
        Restart-Service -Name winrm -Force
    }
    catch {
        Write-Info "Failed to restart WinRM service: $($_.Exception.Message)"
    }

    try {
        $ruleName = "Vagrant WinRM"
        if (-not (Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue)) {
            Write-Info "Creating firewall rule '$ruleName' for WinRM on port 5985 (all profiles)."
            New-NetFirewallRule -DisplayName $ruleName -Name "VagrantWinRM" `
                -Direction Inbound -Action Allow -Protocol TCP -LocalPort 5985 -Profile Any | Out-Null
        }
    }
    catch {
        Write-Info "Failed to configure WinRM firewall rule: $($_.Exception.Message)"
    }
}

function Install-VagrantInsecureKey {
    $sshDir = "C:\Users\vagrant\.ssh"
    $authorizedKeys = Join-Path $sshDir "authorized_keys"
    $publicKey = @"
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC0uE0CEd67+y+/4wndK7opBpZyQ9f5p2AWiPlFVs4Rc68kDireWbhepSvRjodwypMBJgZFDrKQMGMowlS8Mad76HlLIwAS1Ir+raqiJLjext/PGiMi3qBG8zf3M1h6Bu8zwmNCCNRj5t9v32qfHSYqacFBqRQw8dHV4v/TofDADYFtK9CX7Dmwvat1vas2JwSnbn24feA8JCHD1X6jR8pPzMrDQv8dGzQ+QyK6bC+DoKfpnmV0mnChk+fSdrptyPpAH4UFSTofque2ozKnDN8lRiORjyMkI13ANmfxjbTxbwh/tJ2sys5VNDH1Mc+qT1VoJFVBw4P6w0EbUnHcwEtV3J vagrant insecure public key
"@

    if (-not (Test-Path $sshDir)) {
        New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
    }

    if (Test-Path $authorizedKeys) {
        $existing = Get-Content -Path $authorizedKeys -ErrorAction SilentlyContinue
        if ($existing -contains $publicKey.Trim()) {
            Write-Info "Vagrant insecure key already present."
            return
        }
    }

    Write-Info "Installing Vagrant insecure SSH key."
    $publicKey | Out-File -FilePath $authorizedKeys -Encoding ascii -Force

    icacls $sshDir /inheritance:r | Out-Null
    icacls $sshDir /grant:r "SYSTEM:(OI)(CI)F" "Administrators:(OI)(CI)F" "vagrant:(OI)(CI)F" | Out-Null

    icacls $authorizedKeys /inheritance:r | Out-Null
    icacls $authorizedKeys /grant:r "SYSTEM:R" "Administrators:R" "NT SERVICE\SSHD:R" "vagrant:F" | Out-Null
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

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Go
Build-Xp2p
Ensure-OpenSSH
Install-VagrantInsecureKey
Set-PrivateNetworkProfile -AddressPrefixPattern "10.0.10."
Ensure-Xp2pFirewall -Port 62022 -Role $xp2pRole
Configure-WinRM
Run-SmokeTest -Skip:$false

Write-Info "Provisioning completed successfully."
