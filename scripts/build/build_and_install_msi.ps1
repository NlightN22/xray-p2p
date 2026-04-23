param(
    [string] $RepoRoot = 'C:\xp2p',
    [string] $CacheDir = 'C:\xp2p\build\msi-cache',
    [string] $WixSourceRelative = 'installer\wix\xp2p.wxs',
    [string] $MsiArchLabel = 'amd64',
    [ValidateSet('native', 'dotnet')]
    [string] $UiBackend = 'native',
    [switch] $UiSelfContained = $false,
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

function Ensure-DotNetSdk {
    if (Get-Command -Name dotnet.exe -ErrorAction SilentlyContinue) {
        return
    }

    $candidates = @(
        "C:\Program Files\dotnet\dotnet.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            Add-ToPath (Split-Path $candidate -Parent)
            break
        }
    }

    if (-not (Get-Command -Name dotnet.exe -ErrorAction SilentlyContinue)) {
        throw "dotnet.exe not found. Install .NET SDK or ensure it is on PATH."
    }
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

function Resolve-UiRuntimeId {
    param([string] $ArchLabel)

    switch ($ArchLabel) {
        "amd64" { return "win-x64" }
        "x86" { return "win-x86" }
        default { throw "Unsupported UI runtime arch '$ArchLabel'." }
    }
}

function Resolve-CMakePlatform {
    param([string] $ArchLabel)

    switch ($ArchLabel) {
        "amd64" { return "x64" }
        "x86" { return "Win32" }
        default { return "" }
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
if ($UiBackend -eq 'dotnet') {
    Ensure-DotNetSdk
}
else {
    Ensure-CMake
}

$version = Get-Xp2pVersion -RepoRoot $RepoRoot
$msiPath = Join-Path $CacheDir ("xp2p-$version-windows-$MsiArchLabel.msi")

Push-Location $RepoRoot
try {
    $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
    $binaryDir = Join-Path $RepoRoot 'build\msi-bin'

    Write-Info "Cleaning previous build artifacts"
    Remove-Item $binaryDir -Recurse -Force -ErrorAction SilentlyContinue
    Ensure-Directory $binaryDir

    $xp2pRsrcPrefix = Join-Path $RepoRoot 'go\cmd\xp2p\rsrc'

    Invoke-Step -Name "Generating Windows version resources" -Action {
        $skipWinres = $env:XP2P_SKIP_WINRES
        $forceWinres = $env:XP2P_FORCE_WINRES

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

            $existingWinres = Get-ChildItem "$RsrcPrefix*_windows_*.syso" -ErrorAction SilentlyContinue
            $shouldRunWinres = $true
            if ($skipWinres -and $skipWinres -ne "0") {
                $shouldRunWinres = $false
            } elseif ($existingWinres -and $forceWinres -ne "1") {
                $shouldRunWinres = $false
            }

            if ($shouldRunWinres) {
                $goWinres = Invoke-GoCommand -CommandArgs @(
                    "run",
                    "github.com/tc-hib/go-winres@v0.2.0",
                    "make",
                    "--in", $ConfigPath,
                    "--out", $RsrcPrefix,
                    "--arch", $Arch,
                    "--product-version", $version,
                    "--file-version", $version
                )
                $goWinres.Output | ForEach-Object { Write-Host $_ }
                if ($goWinres.ExitCode -ne 0) {
                    Write-Info "go-winres failed for $Label; continuing without refreshed resources."
                }
            } else {
                Write-Info "Skipping winres generation for $Label (existing resources detected)."
            }
        }

        Invoke-WinresMake -ConfigPath (Join-Path $RepoRoot 'scripts\build\winres.json') -RsrcPrefix $xp2pRsrcPrefix -Arch "amd64" -Label "xp2p"
    }

    $binaryOut = Join-Path $binaryDir 'xp2p.exe'
    Invoke-Step -Name "Building xp2p.exe" -Action {
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
    }
    if (-not (Test-Path $binaryOut)) {
        throw "xp2p binary missing at $binaryOut"
    }

    $uiBinaryOut = Join-Path $binaryDir 'ui-xp2p.exe'
    Invoke-Step -Name "Building ui-xp2p.exe" -Action {
        if ($UiBackend -eq 'dotnet') {
            $uiProject = Join-Path $RepoRoot "dotnet\\ui-xp2p\\ui-xp2p.csproj"
            if (-not (Test-Path $uiProject)) {
                throw "WPF UI project not found at $uiProject"
            }
            $uiPublishDir = Join-Path $binaryDir 'ui-xp2p-publish'
            Ensure-Directory $uiPublishDir
            $runtimeId = Resolve-UiRuntimeId -ArchLabel $MsiArchLabel
            Write-Info ("Publishing WPF UI (RID={0}, selfContained={1})" -f $runtimeId, ($UiSelfContained -eq $true))

            $uiPublishProps = @(
                "/p:PublishSingleFile=true",
                "/p:DebugType=None",
                "/p:DebugSymbols=false",
                "/p:PublishReadyToRun=false"
            )
            if ($UiSelfContained) {
                $uiPublishProps += @(
                    "/p:SelfContained=true",
                    "/p:IncludeNativeLibrariesForSelfExtract=true",
                    "/p:EnableCompressionInSingleFile=true"
                )
            }
            else {
                $uiPublishProps += "/p:SelfContained=false"
            }

            $dotnetOutput = & dotnet publish $uiProject -c Release -r $runtimeId @uiPublishProps -o $uiPublishDir 2>&1
            if ($dotnetOutput) {
                $dotnetOutput | ForEach-Object { Write-Host $_ }
            }
            if ($LASTEXITCODE -ne 0) {
                throw "dotnet publish failed with exit code $LASTEXITCODE"
            }
            $publishedExe = Join-Path $uiPublishDir 'ui-xp2p.exe'
            if (-not (Test-Path $publishedExe)) {
                throw "WPF UI publish output missing at $publishedExe"
            }
            if (-not $UiSelfContained) {
                $extraFiles = @(Get-ChildItem $uiPublishDir -File | Where-Object { $_.Name -ne 'ui-xp2p.exe' })
                if ($extraFiles.Count -gt 0) {
                    $names = ($extraFiles | Select-Object -ExpandProperty Name) -join ", "
                    throw "Framework-dependent UI publish produced extra files: $names. MSI currently embeds only ui-xp2p.exe; use -UiSelfContained or update packaging."
                }
            }
            Copy-Item -Path $publishedExe -Destination $uiBinaryOut -Force
            return
        }

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
        if (-not (Test-Path $binaryOut)) {
            Write-Info "xp2p.exe missing before completion step; rebuilding."
            $goBuild = Invoke-GoCommand -CommandArgs @(
                "build",
                "-trimpath",
                "-ldflags", $ldflags,
                "-o", $binaryOut,
                ".\\go\\cmd\\xp2p"
            )
            $goBuild.Output | ForEach-Object { Write-Host $_ }
            if ($goBuild.ExitCode -ne 0) {
                throw "go build failed while restoring xp2p.exe (exit code $($goBuild.ExitCode))"
            }
        }
        if (-not (Test-Path $binaryOut)) {
            throw "xp2p binary missing at $binaryOut before completion step"
        }
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

    $bundleSourceDir = Join-Path $RepoRoot 'distro\windows\bundle\x86_64'
    $bundleSourceXray = Join-Path $bundleSourceDir 'xray.exe'
    if (-not (Test-Path $bundleSourceXray)) {
        throw "xray binary missing at $bundleSourceXray (place the Windows bundle before building the MSI)."
    }
    $bundleDir = Join-Path $binaryDir 'bundle'
    Ensure-Directory $bundleDir
    Invoke-Step -Name "Copying xray bundle" -Action {
        Get-ChildItem -Path $bundleSourceDir -File | Where-Object { $_.Name -ne '.gitkeep' } | ForEach-Object {
            Copy-Item $_.FullName $bundleDir -Force
        }
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
        $bundleContent = Get-Content -Path $bundleWxs -Raw
        $bundleContent = [regex]::Replace($bundleContent, '<Component(\s+)(?![^>]*\bWin64=)', '<Component Win64="yes"$1')
        Set-Content -Path $bundleWxs -Value $bundleContent
    }

    $wixObj = Join-Path $binaryDir 'xp2p.wixobj'
    $bundleObj = Join-Path $binaryDir 'xp2p-bundle.wixobj'
    $registerPsCompletion = Join-Path $RepoRoot 'installer\wix\register_ps_completion.ps1'
    $setServiceAclScript = Join-Path $RepoRoot 'installer\wix\set_service_acl.ps1'
    Invoke-Step -Name "Running candle.exe (main wixobj)" -Action {
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
    Invoke-Step -Name "Running candle.exe (bundle wixobj)" -Action {
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

    Invoke-Step -Name "Running light.exe" -Action {
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
