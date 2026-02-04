param(
    [string] $RepoRoot = 'C:\xp2p',
    [string] $CacheDir = 'C:\xp2p\build\msi-cache',
    [string] $WixSourceRelative = 'installer\wix\xp2p.wxs',
    [string] $MsiArchLabel = 'amd64',
    [switch] $BuildOnly = $false,
    [string] $OutputMarker = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )
    Write-Host "==> $Message"
}

function Ensure-Directory {
    param([string] $Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Add-ToPath {
    param([string] $Path)
    $current = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $segments = $current -split ';'
    if ($segments -contains $Path) {
        return
    }
    $newPath = "$Path;$current"
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
    $env:Path = "$Path;$env:Path"
}

function Invoke-GoCommand {
    param([string[]] $CommandArgs)

    $cmdArgs = @($CommandArgs | Where-Object { $_ -ne $null -and $_ -ne "" })
    if ($cmdArgs.Count -eq 0) {
        throw "Invoke-GoCommand called with empty arguments."
    }
    Write-Info ("Running go {0}" -f ($cmdArgs -join " "))

    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $output = & go @cmdArgs 2>&1
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $prev
    return @{
        Output = $output
        ExitCode = $exitCode
    }
}

function Ensure-GoToolchain {
    if (Get-Command -Name go.exe -ErrorAction SilentlyContinue) {
        return
    }

    $candidates = @(
        "C:\Program Files\Go\bin\go.exe",
        "C:\tools\go\bin\go.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            Add-ToPath (Split-Path $candidate -Parent)
            break
        }
    }

    if (-not (Get-Command -Name go.exe -ErrorAction SilentlyContinue)) {
        throw "go.exe not found. Install Go or ensure it is on PATH."
    }
}

function Get-Xp2pVersion {
    param([string] $RepoRoot)

    $versionFile = Join-Path $RepoRoot 'go\internal\version\version.go'
    if (-not (Test-Path $versionFile)) {
        throw "Version file not found at $versionFile"
    }

    $match = Select-String -Path $versionFile -Pattern 'var\s+current\s*=\s*"([^"]+)"' -AllMatches |
        Select-Object -First 1
    if (-not $match) {
        throw "Unable to parse xp2p version from $versionFile"
    }

    return $match.Matches[0].Groups[1].Value
}

Write-Info "Preparing MSI build directories"
Ensure-Directory $RepoRoot
Ensure-Directory $CacheDir
Ensure-GoToolchain

$version = Get-Xp2pVersion -RepoRoot $RepoRoot
$msiPath = Join-Path $CacheDir ("xp2p-$version-windows-$MsiArchLabel.msi")

Push-Location $RepoRoot
try {
    $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
    $binaryDir = Join-Path $RepoRoot 'build\msi-bin'

    Write-Info "Cleaning previous build artifacts"
    Remove-Item $binaryDir -Recurse -Force -ErrorAction SilentlyContinue
    Ensure-Directory $binaryDir

    Write-Info "Generating Windows version resources"
    $winresConfig = Join-Path $RepoRoot 'scripts\build\winres.json'
    if (-not (Test-Path $winresConfig)) {
        throw "winres config missing at $winresConfig"
    }
    $rsrcPrefix = Join-Path $RepoRoot 'go\cmd\xp2p\rsrc'
    Get-ChildItem "$rsrcPrefix*_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force
    $goWinres = Invoke-GoCommand -CommandArgs @(
        "run",
        "github.com/tc-hib/go-winres@v0.2.0",
        "make",
        "--in", $winresConfig,
        "--out", $rsrcPrefix,
        "--arch", "amd64",
        "--product-version", $version,
        "--file-version", $version
    )
    $goWinres.Output | ForEach-Object { Write-Host $_ }
    if ($goWinres.ExitCode -ne 0) {
        throw "go-winres failed with exit code $($goWinres.ExitCode)"
    }

    Write-Info "Building xp2p.exe"
    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    $goBuild = Invoke-GoCommand -CommandArgs @(
        "build",
        "-trimpath",
        "-ldflags", $ldflags,
        "-o", $binaryOut,
        ".\\go\\cmd\\xp2p"
    )
    $goBuild.Output | ForEach-Object { Write-Host $_ }
    if ($goBuild.ExitCode -ne 0) {
        throw "go build failed with exit code $($goBuild.ExitCode)"
    }

    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    $bundleSourceDir = Join-Path $RepoRoot 'distro\windows\bundle\x86_64'
    $bundleSourceXray = Join-Path $bundleSourceDir 'xray.exe'
    if (-not (Test-Path $bundleSourceXray)) {
        throw "xray binary missing at $bundleSourceXray (place the Windows bundle before building the MSI)."
    }
    $bundleDir = Join-Path $binaryDir 'bundle'
    Ensure-Directory $bundleDir
    Get-ChildItem -Path $bundleSourceDir -File | Where-Object { $_.Name -ne '.gitkeep' } | ForEach-Object {
        Copy-Item $_.FullName $bundleDir -Force
    }

    Write-Info "Locating WiX Toolset"
    $wixDir = Get-ChildItem "C:\Program Files (x86)" -Filter "WiX Toolset*" -Directory |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $wixDir) {
        throw "WiX Toolset installation directory not found."
    }
    $candle = Join-Path $wixDir.FullName 'bin\candle.exe'
    $heat = Join-Path $wixDir.FullName 'bin\heat.exe'
    $light = Join-Path $wixDir.FullName 'bin\light.exe'

    Write-Info "Harvesting xray bundle"
    $bundleWxs = Join-Path $binaryDir 'xp2p-bundle.wxs'
    & $heat dir $bundleDir -dr BinFolder -cg Xp2pBundleGroup -gg -srd -var var.BundleDir -out $bundleWxs
    if ($LASTEXITCODE -ne 0) {
        throw "heat.exe failed with exit code $LASTEXITCODE"
    }
    $bundleContent = Get-Content -Path $bundleWxs -Raw
    $bundleContent = [regex]::Replace($bundleContent, '<Component(\s+)(?![^>]*\bWin64=)', '<Component Win64="yes"$1')
    Set-Content -Path $bundleWxs -Value $bundleContent

    Write-Info "Running candle.exe"
    $wixObj = Join-Path $binaryDir 'xp2p.wixobj'
    $bundleObj = Join-Path $binaryDir 'xp2p-bundle.wixobj'
    & $candle "-dProductVersion=$version" "-dXp2pBinary=$binaryOut" "-dBundleDir=$bundleDir" "-out" $wixObj (Join-Path $RepoRoot $WixSourceRelative)
    if ($LASTEXITCODE -ne 0) {
        throw "candle.exe failed with exit code $LASTEXITCODE"
    }
    & $candle "-dProductVersion=$version" "-dXp2pBinary=$binaryOut" "-dBundleDir=$bundleDir" "-out" $bundleObj $bundleWxs
    if ($LASTEXITCODE -ne 0) {
        throw "candle.exe failed with exit code $LASTEXITCODE"
    }

    Write-Info "Running light.exe"
    & $light "-out" $msiPath $wixObj $bundleObj
    if ($LASTEXITCODE -ne 0) {
        throw "light.exe failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $msiPath)) {
    throw "MSI build failed - file not found at $msiPath"
}

if (-not $BuildOnly) {
    Write-Info "Installing xp2p from MSI"
    Start-Process -FilePath 'msiexec.exe' -ArgumentList '/i', "`"$msiPath`"", '/qn', '/norestart' -Wait

    $installDir = Join-Path $env:ProgramFiles 'xp2p'
    Write-Info "Ensuring $installDir is on PATH"
    Add-ToPath $installDir

    Write-Info "xp2p MSI build and installation complete"
}
else {
    Write-Info "xp2p MSI build complete (build-only mode)"
}

Write-Info "MSI path: $msiPath"
if ($OutputMarker) {
    Write-Output ("$OutputMarker$msiPath")
}
