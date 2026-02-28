param(
    [string] $RepoRoot = 'C:\xp2p',
    [string] $CacheDir = 'C:\xp2p\build\msi-cache-x86',
    [string] $WixSourceRelative = 'installer\wix\xp2p-x86.wxs',
    [string] $MsiArchLabel = 'x86',
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

function Invoke-WixTool {
    param(
        [Parameter(Mandatory = $true)]
        [string] $ToolPath,
        [Parameter(Mandatory = $true)]
        [string[]] $Arguments
    )
    $output = & $ToolPath @Arguments 2>&1
    $exitCode = $LASTEXITCODE
    if ($output) {
        $output | ForEach-Object { Write-Host $_ }
    }
    return $exitCode
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

Write-Info "Preparing MSI (x86) build directories"
Ensure-Directory $RepoRoot
Ensure-Directory $CacheDir

Push-Location $RepoRoot
$msiPath = $null
try {
    Write-Info "Resolving xp2p version"
    $version = (& go run .\go\cmd\xp2p --version).Trim()
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "xp2p --version returned empty output."
    }

    $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
    $binaryDir = Join-Path $RepoRoot 'build\msi-bin-x86'
    $msiPath = Join-Path $CacheDir ("xp2p-$version-windows-$MsiArchLabel.msi")

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
    go run github.com/tc-hib/go-winres@v0.2.0 make --in $winresConfig --out $rsrcPrefix --arch 386 --product-version $version --file-version $version
    if ($LASTEXITCODE -ne 0) {
        throw "go-winres failed with exit code $LASTEXITCODE"
    }

    Write-Info "Building xp2p.exe (x86)"
    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    $env:GOARCH = '386'
    $env:GOOS = 'windows'
    go build -trimpath -ldflags $ldflags -o $binaryOut .\go\cmd\xp2p
    Remove-Item Env:GOARCH
    Remove-Item Env:GOOS

    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    Write-Info "Generating PowerShell completion script"
    $completionDir = Join-Path $binaryDir 'completions'
    Ensure-Directory $completionDir
    $completionOut = Join-Path $completionDir 'xp2p.ps1'
    & $binaryOut completion powershell |
        Where-Object { $_ -notmatch '^\d{4}-\d{2}-\d{2}T.*\bxp2p:' } |
        Set-Content -Path $completionOut -Encoding utf8
    if ($LASTEXITCODE -ne 0) {
        throw "xp2p completion powershell failed with exit code $LASTEXITCODE"
    }
    if (-not (Test-Path $completionOut)) {
        throw "PowerShell completion script missing at $completionOut"
    }

    $bundleSourceDir = Join-Path $RepoRoot 'distro\windows\bundle\x86'
    $bundleSourceXray = Join-Path $bundleSourceDir 'xray.exe'
    if (-not (Test-Path $bundleSourceXray)) {
        throw "xray binary missing at $bundleSourceXray (place the 32-bit bundle before building the MSI)."
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
    $wixExt = Join-Path $wixDir.FullName 'bin\WixUtilExtension.dll'

    Write-Info "Harvesting xray bundle"
    $bundleWxs = Join-Path $binaryDir 'xp2p-bundle.wxs'
    $heatExit = Invoke-WixTool -ToolPath $heat -Arguments @(
        "dir", $bundleDir,
        "-dr", "BinFolder",
        "-cg", "Xp2pBundleGroup",
        "-gg",
        "-srd",
        "-var", "var.BundleDir",
        "-out", $bundleWxs
    )
    if ($heatExit -ne 0) {
        throw "heat.exe failed with exit code $heatExit"
    }

    Write-Info "Running candle.exe (x86)"
    $wixObj = Join-Path $binaryDir 'xp2p-x86.wixobj'
    $bundleObj = Join-Path $binaryDir 'xp2p-x86-bundle.wixobj'
    $registerPsCompletion = Join-Path $RepoRoot 'installer\wix\register_ps_completion.ps1'
    $candleExit = Invoke-WixTool -ToolPath $candle -Arguments @(
        "-ext", $wixExt,
        "-dProductVersion=$version",
        "-dXp2pBinary=$binaryOut",
        "-dBundleDir=$bundleDir",
        "-dXp2pCompletionScript=$completionOut",
        "-dRegisterPsCompletionScript=$registerPsCompletion",
        "-out", $wixObj,
        (Join-Path $RepoRoot $WixSourceRelative)
    )
    if ($candleExit -ne 0) {
        throw "candle.exe failed with exit code $candleExit"
    }
    $candleBundleExit = Invoke-WixTool -ToolPath $candle -Arguments @(
        "-ext", $wixExt,
        "-dProductVersion=$version",
        "-dXp2pBinary=$binaryOut",
        "-dBundleDir=$bundleDir",
        "-dXp2pCompletionScript=$completionOut",
        "-dRegisterPsCompletionScript=$registerPsCompletion",
        "-out", $bundleObj,
        $bundleWxs
    )
    if ($candleBundleExit -ne 0) {
        throw "candle.exe failed with exit code $candleBundleExit"
    }

    Write-Info "Running light.exe (x86)"
    $lightExit = Invoke-WixTool -ToolPath $light -Arguments @(
        "-ext", $wixExt,
        "-out", $msiPath,
        $wixObj,
        $bundleObj
    )
    if ($lightExit -ne 0) {
        throw "light.exe failed with exit code $lightExit"
    }
}
finally {
    Pop-Location
}

if (-not (Test-Path $msiPath)) {
    throw "MSI build failed - file not found at $msiPath"
}

if (-not $BuildOnly) {
    Write-Info "Installing xp2p (x86) from MSI"
    Start-Process -FilePath 'msiexec.exe' -ArgumentList '/i', "`"$msiPath`"", '/qn', '/norestart' -Wait

    $installDir = Join-Path ${env:ProgramFiles(x86)} 'xp2p'
    if (-not (Test-Path $installDir)) {
        $installDir = Join-Path $env:ProgramFiles 'xp2p'
    }
    Write-Info "Ensuring $installDir is on PATH"
    Add-ToPath $installDir

    Write-Info "xp2p MSI (x86) build and installation complete"
}
else {
    Write-Info "xp2p MSI (x86) build complete (build-only mode)"
}

Write-Info "MSI path: $msiPath"
if ($OutputMarker) {
    Write-Output ("$OutputMarker$msiPath")
}
