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
        $attempt = 0
        do {
            $attempt++
            choco @installArgs | Write-Host
            $code = $LASTEXITCODE
            if ($code -eq 0) {
                break
            }
            Write-Info ("Chocolatey install failed for '{0}' (exit={1}, attempt={2}/3)" -f $Package, $code, $attempt)
            Start-Sleep -Seconds (5 * $attempt)
        } while ($attempt -lt 3)
        if ($code -ne 0) {
            throw "Chocolatey failed to install package '$Package' (exit code $code)."
        }
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

function Disable-WindowsAutoUpdate {
    Write-Info "Disabling Windows Update automatic checks."

    $policyPath = "HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU"
    try {
        if (-not (Test-Path $policyPath)) {
            New-Item -Path $policyPath -Force | Out-Null
        }
        New-ItemProperty -Path $policyPath -Name "NoAutoUpdate" -Value 1 -PropertyType DWord -Force | Out-Null
        New-ItemProperty -Path $policyPath -Name "AUOptions" -Value 2 -PropertyType DWord -Force | Out-Null
    }
    catch {
        Write-Info ("Failed to update Windows Update policy registry: {0}" -f $_.Exception.Message)
    }

    $services = @("wuauserv", "UsoSvc")
    foreach ($service in $services) {
        try {
            $svc = Get-Service -Name $service -ErrorAction Stop
            if ($svc.Status -ne "Stopped") {
                Stop-Service -Name $service -Force -ErrorAction Stop
            }
            Set-Service -Name $service -StartupType Disabled -ErrorAction Stop
        }
        catch {
            Write-Info ("Failed to disable service {0}: {1}" -f $service, $_.Exception.Message)
        }
    }
}

function Ensure-WindowsInstallerEnabled {
    Write-Info "Ensuring Windows Installer policies allow MSI installs."

    $policyRoots = @(
        "HKLM:\SOFTWARE\Policies\Microsoft\Windows\Installer",
        "HKCU:\SOFTWARE\Policies\Microsoft\Windows\Installer"
    )

    foreach ($root in $policyRoots) {
        try {
            if (-not (Test-Path $root)) {
                New-Item -Path $root -Force | Out-Null
            }
            New-ItemProperty -Path $root -Name "DisableMSI" -Value 0 -PropertyType DWord -Force | Out-Null
        }
        catch {
            Write-Info ("Failed to update MSI policy at {0}: {1}" -f $root, $_.Exception.Message)
        }
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

function Ensure-DefenderExclusions {
    Write-Info "Ensuring Microsoft Defender exclusions for xp2p test artifacts."

    $addExclusion = Get-Command -Name Add-MpPreference -ErrorAction SilentlyContinue
    if (-not $addExclusion) {
        Write-Info "Add-MpPreference is not available; skipping Defender exclusions."
        return
    }

    $paths = @(
        "C:\\xp2p",
        "C:\\ProgramData\\xp2p",
        "C:\\Program Files\\xp2p"
    )
    foreach ($path in $paths) {
        try {
            Add-MpPreference -ExclusionPath $path | Out-Null
            Write-Info ("Added Defender path exclusion: {0}" -f $path)
        }
        catch {
            Write-Info ("Failed to add Defender path exclusion '{0}': {1}" -f $path, $_.Exception.Message)
        }
    }

    $processes = @("xp2p.exe", "xray.exe")
    foreach ($proc in $processes) {
        try {
            Add-MpPreference -ExclusionProcess $proc | Out-Null
            Write-Info ("Added Defender process exclusion: {0}" -f $proc)
        }
        catch {
            Write-Info ("Failed to add Defender process exclusion '{0}': {1}" -f $proc, $_.Exception.Message)
        }
    }
}

function Ensure-CMakeToolchain {
    $cmakeVersion = $env:XP2P_CMAKE_VERSION
    if (-not $cmakeVersion) {
        $cmakeVersion = $null
    }

    $ninjaVersion = $env:XP2P_NINJA_VERSION
    if (-not $ninjaVersion) {
        $ninjaVersion = $null
    }

    $vsBuildToolsVersion = $env:XP2P_VS_BUILD_TOOLS_VERSION
    if (-not $vsBuildToolsVersion) {
        $vsBuildToolsVersion = $null
    }

    Ensure-ChocoPackage -Package "cmake" -Version $cmakeVersion
    Ensure-ChocoPackage -Package "ninja" -Version $ninjaVersion
    Ensure-ChocoPackage -Package "visualstudio2022buildtools" -Version $vsBuildToolsVersion

    $hasCl = Test-Path "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC\*\bin\Hostx64\x64\cl.exe"
    if (-not $hasCl) {
        Write-Info "MSVC C++ tools not detected; installing VC Tools workload package."
        $installArgs = @("install", "visualstudio2022-workload-vctools", "--yes", "--no-progress")
        choco @installArgs | Write-Host
        if ($LASTEXITCODE -ne 0) {
            throw "Chocolatey failed to install Visual Studio VC Tools workload (exit code $LASTEXITCODE)."
        }
    }

    $cmakeExeCandidates = @(
        "C:\Program Files\CMake\bin\cmake.exe",
        "C:\Program Files (x86)\CMake\bin\cmake.exe",
        "C:\ProgramData\chocolatey\bin\cmake.exe"
    )
    $ninjaExeCandidates = @(
        "C:\ProgramData\chocolatey\bin\ninja.exe",
        "C:\ProgramData\chocolatey\lib\ninja\tools\ninja.exe"
    )

    function Resolve-ExistingPath {
        param([string[]] $Candidates)
        foreach ($candidate in $Candidates) {
            if ($candidate -and (Test-Path $candidate)) {
                return $candidate
            }
        }
        return $null
    }

    if (-not (Get-Command -Name cmake.exe -ErrorAction SilentlyContinue)) {
        $cmakeExe = Resolve-ExistingPath -Candidates $cmakeExeCandidates
        if ($cmakeExe) {
            $cmakeBin = Split-Path -Parent $cmakeExe
            Write-Info "Adding CMake path '$cmakeBin' to the current session PATH."
            $env:Path = "$cmakeBin;$env:Path"
        }
    }

    if (-not (Get-Command -Name ninja.exe -ErrorAction SilentlyContinue)) {
        $ninjaExe = Resolve-ExistingPath -Candidates $ninjaExeCandidates
        if ($ninjaExe) {
            $ninjaBin = Split-Path -Parent $ninjaExe
            Write-Info "Adding Ninja path '$ninjaBin' to the current session PATH."
            $env:Path = "$ninjaBin;$env:Path"
        }
    }

    if (-not (Get-Command -Name cmake.exe -ErrorAction SilentlyContinue)) {
        $existing = Resolve-ExistingPath -Candidates $cmakeExeCandidates
        if ($existing) {
            throw "cmake.exe exists at '$existing' but is not discoverable via PATH."
        }
        throw "cmake.exe not found after installation. Ensure CMake is available."
    }
    if (-not (Get-Command -Name ninja.exe -ErrorAction SilentlyContinue)) {
        $existing = Resolve-ExistingPath -Candidates $ninjaExeCandidates
        if ($existing) {
            throw "ninja.exe exists at '$existing' but is not discoverable via PATH."
        }
        throw "ninja.exe not found after installation. Ensure Ninja is available."
    }

    $vswhereCandidates = @(
        "C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe",
        "C:\Program Files\Microsoft Visual Studio\Installer\vswhere.exe"
    )
    $vswhere = $vswhereCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $vswhere) {
        Write-Info "vswhere.exe not found; CMake will attempt to locate MSVC via registry."
    }
    else {
        Write-Info ("vswhere ready: {0}" -f $vswhere)
    }

    $msbuildCandidates = @(
        "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\MSBuild.exe",
        "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\MSBuild.exe"
    )
    $msbuild = $msbuildCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $msbuild) {
        Write-Info "MSBuild.exe not found at expected locations; Visual Studio Build Tools installation may be incomplete."
    }
    else {
        Write-Info ("MSBuild ready: {0}" -f $msbuild)
    }

    $cmakeInfo = & cmake.exe --version | Select-Object -First 1
    $ninjaInfo = & ninja.exe --version 2>$null
    Write-Info ("CMake ready: {0}" -f $cmakeInfo)
    if ($ninjaInfo) {
        Write-Info ("Ninja ready: {0}" -f $ninjaInfo)
    }
    Write-Info "MSVC build tools provisioned."
}

Write-Info "Provisioning role detected."

Ensure-IsElevated
Ensure-Chocolatey
Ensure-Go
Ensure-CMakeToolchain
Ensure-WiX
Ensure-WindowsInstallerEnabled
Ensure-DefenderExclusions
Disable-WindowsAutoUpdate
Disable-IdleSleepAndHibernate
Write-Info "Network configuration handled by network_setup.ps1."
Write-Info "Provisioning completed successfully."
