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
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Write-Host ("==> [{0}] {1}" -f $ts, $Message)
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

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Name,
        [Parameter(Mandatory = $true)]
        [scriptblock] $Action
    )
    $start = Get-Date
    Write-Info "$Name (start)"
    & $Action
    $end = Get-Date
    $seconds = [math]::Round(($end - $start).TotalSeconds, 2)
    Write-Info ("$Name (done in {0}s)" -f $seconds)
}

function Remove-WinresArtifacts {
    param([string] $RsrcPrefix, [string] $Label)
    $pattern = "$RsrcPrefix*_windows_*.syso"
    Get-ChildItem $pattern -ErrorAction SilentlyContinue | Remove-Item -Force
    Write-Info ("Cleaned winres artifacts for {0}" -f $Label)
}

Write-Info "Preparing MSI (x86) build directories"
Ensure-Directory $RepoRoot
Ensure-Directory $CacheDir

Push-Location $RepoRoot
$msiPath = $null
$xp2pRsrcPrefix = Join-Path $RepoRoot 'go\cmd\xp2p\rsrc'
$xp2pUiRsrcPrefix = Join-Path $RepoRoot 'go\cmd\xp2p-ui\rsrc'
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

    Invoke-Step -Name "Generating Windows version resources" -Action {
        function Invoke-WinresMake {
            param(
                [Parameter(Mandatory = $true)]
                [string] $ConfigPath,
                [Parameter(Mandatory = $true)]
                [string] $RsrcPrefix,
                [Parameter(Mandatory = $true)]
                [string] $Arch,
                [Parameter(Mandatory = $true)]
                [string] $Label
            )

            if (-not (Test-Path $ConfigPath)) {
                throw "winres config missing at $ConfigPath"
            }

            Get-ChildItem "$RsrcPrefix*_windows_*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force
            go run github.com/tc-hib/go-winres@v0.2.0 make --in $ConfigPath --out $RsrcPrefix --arch $Arch --product-version $version --file-version $version
            if ($LASTEXITCODE -ne 0) {
                throw "go-winres failed for $Label with exit code $LASTEXITCODE"
            }
        }

        Invoke-WinresMake -ConfigPath (Join-Path $RepoRoot 'scripts\build\winres.json') -RsrcPrefix $xp2pRsrcPrefix -Arch "386" -Label "xp2p"
        Invoke-WinresMake -ConfigPath (Join-Path $RepoRoot 'scripts\build\winres-ui.json') -RsrcPrefix $xp2pUiRsrcPrefix -Arch "386" -Label "xp2p-ui"
    }

    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    Invoke-Step -Name "Building xp2p.exe (x86)" -Action {
        $env:GOARCH = '386'
        $env:GOOS = 'windows'
        go build -trimpath -ldflags $ldflags -o $binaryOut .\go\cmd\xp2p
        Remove-Item Env:GOARCH
        Remove-Item Env:GOOS
    }
    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    $uiBinaryOut = Join-Path $binaryDir 'xp2p-ui.exe'
    Invoke-Step -Name "Building xp2p-ui.exe (x86)" -Action {
        $env:GOARCH = '386'
        $env:GOOS = 'windows'
        go build -trimpath -tags production -ldflags "$ldflags -H=windowsgui" -o $uiBinaryOut .\go\cmd\xp2p-ui
        Remove-Item Env:GOARCH
        Remove-Item Env:GOOS
    }
    if (-not (Test-Path $uiBinaryOut)) {
        throw "xp2p-ui binary missing at $uiBinaryOut"
    }

    Invoke-Step -Name "Cleaning winres artifacts" -Action {
        Remove-WinresArtifacts -RsrcPrefix $xp2pRsrcPrefix -Label "xp2p"
        Remove-WinresArtifacts -RsrcPrefix $xp2pUiRsrcPrefix -Label "xp2p-ui"
    }

    $completionDir = Join-Path $binaryDir 'completions'
    $completionOut = Join-Path $completionDir 'xp2p.ps1'
    Invoke-Step -Name "Generating PowerShell completion script" -Action {
        Ensure-Directory $completionDir
        & $binaryOut completion powershell |
            Where-Object { $_ -notmatch '^\d{4}-\d{2}-\d{2}T.*\bxp2p:' } |
            Set-Content -Path $completionOut -Encoding utf8
        if ($LASTEXITCODE -ne 0) {
            throw "xp2p completion powershell failed with exit code $LASTEXITCODE"
        }
        if (-not (Test-Path $completionOut)) {
            throw "PowerShell completion script missing at $completionOut"
        }
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
    $wixUiExt = Join-Path $wixDir.FullName 'bin\WixUIExtension.dll'
    Write-Info ("WiX tools: candle={0}, light={1}, heat={2}" -f $candle, $light, $heat)

    $bundleWxs = Join-Path $binaryDir 'xp2p-bundle.wxs'
    Invoke-Step -Name "Harvesting xray bundle (heat.exe)" -Action {
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
    }

    $wixObj = Join-Path $binaryDir 'xp2p-x86.wixobj'
    $bundleObj = Join-Path $binaryDir 'xp2p-x86-bundle.wixobj'
    $registerPsCompletion = Join-Path $RepoRoot 'installer\wix\register_ps_completion.ps1'
    $setServiceAclScript = Join-Path $RepoRoot 'installer\wix\set_service_acl.ps1'
    Invoke-Step -Name "Running candle.exe (x86 main wixobj)" -Action {
        $candleExit = Invoke-WixTool -ToolPath $candle -Arguments @(
            "-ext", $wixExt,
            "-ext", $wixUiExt,
            "-dProductVersion=$version",
            "-dXp2pBinary=$binaryOut",
            "-dXp2pUiBinary=$uiBinaryOut",
            "-dBundleDir=$bundleDir",
            "-dXp2pCompletionScript=$completionOut",
            "-dRegisterPsCompletionScript=$registerPsCompletion",
            "-dSetServiceAclScript=$setServiceAclScript",
            "-out", $wixObj,
            (Join-Path $RepoRoot $WixSourceRelative)
        )
        if ($candleExit -ne 0) {
            throw "candle.exe failed with exit code $candleExit"
        }
    }
    Invoke-Step -Name "Running candle.exe (x86 bundle wixobj)" -Action {
        $candleBundleExit = Invoke-WixTool -ToolPath $candle -Arguments @(
            "-ext", $wixExt,
            "-ext", $wixUiExt,
            "-dProductVersion=$version",
            "-dXp2pBinary=$binaryOut",
            "-dXp2pUiBinary=$uiBinaryOut",
            "-dBundleDir=$bundleDir",
            "-dXp2pCompletionScript=$completionOut",
            "-dRegisterPsCompletionScript=$registerPsCompletion",
            "-dSetServiceAclScript=$setServiceAclScript",
            "-out", $bundleObj,
            $bundleWxs
        )
        if ($candleBundleExit -ne 0) {
            throw "candle.exe failed with exit code $candleBundleExit"
        }
    }

    Invoke-Step -Name "Running light.exe (x86)" -Action {
        $lightExit = Invoke-WixTool -ToolPath $light -Arguments @(
            "-ext", $wixExt,
            "-ext", $wixUiExt,
            "-out", $msiPath,
            $wixObj,
            $bundleObj
        )
        if ($lightExit -ne 0) {
            throw "light.exe failed with exit code $lightExit"
        }
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
