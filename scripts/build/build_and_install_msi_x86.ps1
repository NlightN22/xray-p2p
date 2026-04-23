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

function Ensure-CMake {
    if (Get-Command -Name cmake.exe -ErrorAction SilentlyContinue) {
        return
    }

    $candidates = @(
        "C:\\Program Files\\CMake\\bin\\cmake.exe",
        "C:\\ProgramData\\chocolatey\\bin\\cmake.exe",
        "C:\\ProgramData\\chocolatey\\lib\\cmake.install\\tools\\cmake\\bin\\cmake.exe",
        "C:\\ProgramData\\chocolatey\\lib\\cmake\\tools\\cmake\\bin\\cmake.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            Add-ToPath (Split-Path $candidate -Parent)
            break
        }
    }

    if (-not (Get-Command -Name cmake.exe -ErrorAction SilentlyContinue)) {
        throw "cmake.exe not found. Install CMake or ensure it is on PATH."
    }
}

function Resolve-CMakeGenerator {
    $vswhere = "C:\\Program Files (x86)\\Microsoft Visual Studio\\Installer\\vswhere.exe"
    if (Test-Path $vswhere) {
        $installPath = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath 2>$null
        if ($LASTEXITCODE -eq 0 -and $installPath -and $installPath.ToString().Trim()) {
            return "Visual Studio 17 2022"
        }

        $installPath = & $vswhere -latest -products * -property installationPath 2>$null
        if ($LASTEXITCODE -eq 0 -and $installPath -and $installPath.ToString().Trim()) {
            $cl = Get-ChildItem -Path (Join-Path $installPath "VC\\Tools\\MSVC") -Recurse -Filter cl.exe -ErrorAction SilentlyContinue |
                Where-Object { $_.FullName -match "\\\\bin\\\\Hostx64\\\\x64\\\\cl\\.exe$" } |
                Select-Object -First 1
            if ($cl) {
                return "Visual Studio 17 2022"
            }
        }
    }
    return ""
}

function Invoke-Cmd {
    param(
        [Parameter(Mandatory = $true)]
        [string] $CommandLine
    )
    $oldEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $hadNativePref = $false
    $oldNativePref = $false
    if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -Scope Global -ErrorAction SilentlyContinue) {
        $hadNativePref = $true
        $oldNativePref = $global:PSNativeCommandUseErrorActionPreference
        $global:PSNativeCommandUseErrorActionPreference = $false
    }
    try {
        $output = & cmd.exe /c $CommandLine 2>&1
        $exitCode = $LASTEXITCODE
    }
    finally {
        if ($hadNativePref) {
            $global:PSNativeCommandUseErrorActionPreference = $oldNativePref
        }
        $ErrorActionPreference = $oldEap
    }
    if ($output) {
        $output | ForEach-Object { Write-Host $_ }
    }
    return $exitCode
}

function Resolve-CMakePlatform {
    param([string] $ArchLabel)

    switch ($ArchLabel) {
        "amd64" { return "x64" }
        "x86" { return "Win32" }
        default { return "" }
    }
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
Ensure-CMake

Push-Location $RepoRoot
$msiPath = $null
$xp2pRsrcPrefix = Join-Path $RepoRoot 'go\cmd\xp2p\rsrc'
try {
    Write-Info "Resolving xp2p version"
    $versionResult = Invoke-GoCommand -CommandArgs @("run", ".\\go\\cmd\\xp2p", "--version")
    if ($versionResult.ExitCode -ne 0) {
        throw ("xp2p --version failed with exit code {0}.\n{1}" -f $versionResult.ExitCode, ($versionResult.Output -join "`n"))
    }
    $version = ($versionResult.Output | Where-Object { $_ -ne $null -and $_.ToString().Trim() } | Select-Object -Last 1).ToString().Trim()
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
            $res = Invoke-GoCommand -CommandArgs @(
                "run",
                "github.com/tc-hib/go-winres@v0.2.0",
                "make",
                "--in",
                $ConfigPath,
                "--out",
                $RsrcPrefix,
                "--arch",
                $Arch,
                "--product-version",
                $version,
                "--file-version",
                $version
            )
            if ($res.ExitCode -ne 0) {
                throw ("go-winres failed for {0} with exit code {1}.\n{2}" -f $Label, $res.ExitCode, ($res.Output -join "`n"))
            }
        }

        Invoke-WinresMake -ConfigPath (Join-Path $RepoRoot 'scripts\build\winres.json') -RsrcPrefix $xp2pRsrcPrefix -Arch "386" -Label "xp2p"
    }

    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    Invoke-Step -Name "Building xp2p.exe (x86)" -Action {
        $env:GOARCH = '386'
        $env:GOOS = 'windows'
        $buildResult = Invoke-GoCommand -CommandArgs @("build", "-trimpath", "-ldflags", $ldflags, "-o", $binaryOut, ".\\go\\cmd\\xp2p")
        Remove-Item Env:GOARCH
        Remove-Item Env:GOOS
        if ($buildResult.ExitCode -ne 0) {
            throw ("go build (x86) failed with exit code {0}.\n{1}" -f $buildResult.ExitCode, ($buildResult.Output -join "`n"))
        }
    }
    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    $uiBinaryOut = Join-Path $binaryDir 'ui-xp2p.exe'
    Invoke-Step -Name "Building ui-xp2p.exe (x86)" -Action {
        $nativeUiDir = Join-Path $RepoRoot "cpp\\ui-xp2p"
        $nativeUiCmake = Join-Path $nativeUiDir "CMakeLists.txt"
        if (-not (Test-Path $nativeUiCmake)) {
            throw "Native UI CMake project not found at $nativeUiCmake"
        }
        $nativeBuildDir = Join-Path $binaryDir "ui-xp2p-native-build"
        Ensure-Directory $nativeBuildDir
        $platform = Resolve-CMakePlatform -ArchLabel $MsiArchLabel
        $generator = Resolve-CMakeGenerator
        if (-not $generator) {
            throw "Visual Studio Build Tools (VC Tools workload) not found. MSI packaging requires MSVC to produce a self-contained ui-xp2p.exe (MinGW/Ninja builds may depend on libgcc/libwinpthread DLLs)."
        }
        Write-Info ("Building native UI via CMake (platform={0})" -f $platform)

        $configure = @(
            "cmake",
            "-S", "`"$nativeUiDir`"",
            "-B", "`"$nativeBuildDir`"",
            "-G", "`"$generator`"",
            "-DCMAKE_BUILD_TYPE=Release"
        )
        if ($generator -like "Visual Studio*") {
            if ($platform) {
                $configure += @("-A", "`"$platform`"")
            }
        }

        $configureLine = ($configure -join " ")
        $configureExit = Invoke-Cmd -CommandLine $configureLine
        if ($configureExit -ne 0) {
            throw "cmake configure failed with exit code $configureExit"
        }

        $buildLine = "cmake --build `"$nativeBuildDir`" --config Release"
        $buildExit = Invoke-Cmd -CommandLine $buildLine
        if ($buildExit -ne 0) {
            throw "cmake build failed with exit code $buildExit"
        }

        $candidates = @(
            (Join-Path $nativeBuildDir "Release\\ui-xp2p.exe"),
            (Join-Path $nativeBuildDir "ui-xp2p.exe")
        )
        $builtExe = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
        if (-not $builtExe) {
            throw "Native UI build output missing in $nativeBuildDir"
        }
        Copy-Item -Path $builtExe -Destination $uiBinaryOut -Force
    }
    if (-not (Test-Path $uiBinaryOut)) {
        throw "ui-xp2p binary missing at $uiBinaryOut"
    }

    Invoke-Step -Name "Cleaning winres artifacts" -Action {
        Remove-WinresArtifacts -RsrcPrefix $xp2pRsrcPrefix -Label "xp2p"
    }

    $completionDir = Join-Path $binaryDir 'completions'
    $completionOut = Join-Path $completionDir 'xp2p.ps1'
    Invoke-Step -Name "Generating PowerShell completion script" -Action {
        Ensure-Directory $completionDir
        & $binaryOut completion powershell |
            Where-Object { $_ -notmatch '^\d{4}-\d{2}-\d{2}T.*\b(DEBUG|INFO|WARN|ERROR)\b' } |
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

    Write-Info "Locating WiX Toolset (v3.x)"
    $wixRoot = "C:\Program Files (x86)"
    $wixCandidates = Get-ChildItem $wixRoot -Filter "WiX Toolset*" -Directory -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending
    $wixDir = $null
    foreach ($candidate in $wixCandidates) {
        $bin = Join-Path $candidate.FullName 'bin'
        $hasTools =
            (Test-Path (Join-Path $bin 'candle.exe')) -and
            (Test-Path (Join-Path $bin 'light.exe')) -and
            (Test-Path (Join-Path $bin 'heat.exe')) -and
            (Test-Path (Join-Path $bin 'WixUtilExtension.dll')) -and
            (Test-Path (Join-Path $bin 'WixUIExtension.dll'))
        if ($hasTools) {
            $wixDir = $candidate
            break
        }
    }
    if (-not $wixDir) {
        throw "WiX Toolset v3.x not found under $wixRoot. Install WiX Toolset v3.14 (for example, choco install wixtoolset -y)."
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
