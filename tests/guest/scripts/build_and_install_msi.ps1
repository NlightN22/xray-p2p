param(
    [string] $RepoRoot = 'C:\xp2p',
    [string] $CacheDir = 'C:\xp2p\build\msi-cache'
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

Write-Info "Preparing MSI build directories"
Ensure-Directory $RepoRoot
Ensure-Directory $CacheDir

Push-Location $RepoRoot
try {
    Write-Info "Resolving xp2p version"
    $version = (& go run .\go\cmd\xp2p --version).Trim()
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "xp2p --version returned empty output."
    }

    $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
    $binaryDir = Join-Path $RepoRoot 'build\msi-bin'
    $msiPath = Join-Path $CacheDir ("xp2p-$version-windows-amd64.msi")

    Write-Info "Cleaning previous build artifacts"
    Remove-Item $binaryDir -Recurse -Force -ErrorAction SilentlyContinue
    Ensure-Directory $binaryDir

    Write-Info "Building xp2p.exe"
    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    go build -trimpath -ldflags $ldflags -o $binaryOut .\go\cmd\xp2p

    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    $xraySource = Join-Path $RepoRoot 'distro\windows\bundle\x86_64\xray.exe'
    if (-not (Test-Path $xraySource)) {
        throw "xray binary missing at $xraySource (place the Windows bundle before building the MSI)."
    }
    $xrayOut = Join-Path $binaryDir 'xray.exe'
    Copy-Item $xraySource $xrayOut -Force

    Write-Info "Locating WiX Toolset"
    $wixDir = Get-ChildItem "C:\Program Files (x86)" -Filter "WiX Toolset*" -Directory |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $wixDir) {
        throw "WiX Toolset installation directory not found."
    }
    $candle = Join-Path $wixDir.FullName 'bin\candle.exe'
    $light = Join-Path $wixDir.FullName 'bin\light.exe'

    Write-Info "Running candle.exe"
    $wixObj = Join-Path $binaryDir 'xp2p.wixobj'
    & $candle "-dProductVersion=$version" "-dXp2pBinary=$binaryOut" "-dXrayBinary=$xrayOut" "-out" $wixObj (Join-Path $RepoRoot 'installer\wix\xp2p.wxs')
    if ($LASTEXITCODE -ne 0) {
        throw "candle.exe failed with exit code $LASTEXITCODE"
    }

    Write-Info "Running light.exe"
    & $light "-out" $msiPath $wixObj
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

Write-Info "Installing xp2p from MSI"
Start-Process -FilePath 'msiexec.exe' -ArgumentList '/i', "`"$msiPath`"", '/qn', '/norestart' -Wait

$installDir = Join-Path $env:ProgramFiles 'xp2p'
Write-Info "Ensuring $installDir is on PATH"
Add-ToPath $installDir

Write-Info "xp2p MSI build and installation complete"
Write-Info "MSI path: $msiPath"
