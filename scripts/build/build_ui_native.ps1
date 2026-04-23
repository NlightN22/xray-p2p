param(
    [string]$RepoRoot = "",
    [ValidateSet("build", "test", "cover")]
    [string]$Task = "build",
    [ValidateSet("Release", "Debug")]
    [string]$Config = "Release",
    [ValidateSet("mingw")]
    [string]$Toolchain = "mingw",
    [string]$BuildDir = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    if ($RepoRoot -ne "") {
        return (Resolve-Path $RepoRoot).Path
    }
    return (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path
}

function Ensure-WinLibsToolchain {
    param([string]$Root)

    $tag = "15.2.0posix-14.0.0-ucrt-r7"
    $asset = "winlibs-x86_64-posix-seh-gcc-15.2.0-mingw-w64ucrt-14.0.0-r7.zip"
    $url = "https://github.com/brechtsanders/winlibs_mingw/releases/download/$tag/$asset"

    $toolchainsDir = Join-Path $Root "build\\toolchains"
    $zipPath = Join-Path $toolchainsDir "winlibs-x86_64.zip"
    $extractDir = Join-Path $toolchainsDir "winlibs-x86_64"

    New-Item -ItemType Directory -Force -Path $toolchainsDir | Out-Null

    if (!(Test-Path $zipPath)) {
        Write-Host "==> Downloading portable MinGW toolchain"
        $ProgressPreference = "SilentlyContinue"
        Invoke-WebRequest -Uri $url -OutFile $zipPath
    }

    if (!(Test-Path $extractDir)) {
        Write-Host "==> Extracting toolchain"
        Expand-Archive -Path $zipPath -DestinationPath $extractDir
    }

    $binDir = (Resolve-Path (Join-Path $extractDir "mingw64\\bin")).Path
    return $binDir
}

function To-CMakePath {
    param([string]$Path)
    return $Path.Replace("\", "/")
}

$root = Resolve-RepoRoot
$src = Join-Path $root "cpp\\ui-xp2p"

if ($BuildDir -eq "") {
    if ($Task -eq "build") {
        $BuildDir = "build\\ui-xp2p-host-mingw"
    } else {
        $BuildDir = "build\\ui-xp2p-tests-mingw"
    }
}

$build = Join-Path $root $BuildDir

if ($Toolchain -ne "mingw") {
    throw "Unsupported toolchain: $Toolchain"
}

$tcBin = Ensure-WinLibsToolchain -Root $root
$env:PATH = "$tcBin;$env:PATH"

$gcc = To-CMakePath (Join-Path $tcBin "gcc.exe")
$gxx = To-CMakePath (Join-Path $tcBin "g++.exe")
$windres = To-CMakePath (Join-Path $tcBin "windres.exe")

$cmakeArgs = @(
    "-S", (To-CMakePath $src),
    "-B", (To-CMakePath $build),
    "-G", "Ninja",
    "-DCMAKE_BUILD_TYPE=$Config",
    "-DCMAKE_C_COMPILER=$gcc",
    "-DCMAKE_CXX_COMPILER=$gxx",
    "-DCMAKE_RC_COMPILER=$windres"
)

if ($Task -ne "build") {
    $cmakeArgs += "-DXP2P_UI_BUILD_TESTS=ON"
}
if ($Task -eq "cover") {
    $cmakeArgs += "-DXP2P_UI_ENABLE_COVERAGE=ON"
}

Write-Host "==> Configuring native UI ($Toolchain, $Config)"
& cmake @cmakeArgs

Write-Host "==> Building native UI ($Toolchain, $Config)"
& cmake --build $build --parallel

if ($Task -eq "test" -or $Task -eq "cover") {
    Write-Host "==> Running C++ unit tests"
    & ctest --test-dir $build --output-on-failure
}

if ($Task -eq "cover") {
    $venv = Join-Path $root "build\\venv-gcovr"
    if (!(Test-Path $venv)) {
        & python -m venv $venv
    }
    & (Join-Path $venv "Scripts\\python") -m pip install -q --upgrade pip
    & (Join-Path $venv "Scripts\\python") -m pip install -q gcovr

    $coverageDir = Join-Path $build "coverage"
    New-Item -ItemType Directory -Force -Path $coverageDir | Out-Null

    $gcovFs = (Join-Path $tcBin "gcov.exe")
    if (!(Test-Path $gcovFs)) {
        throw "gcov.exe not found at: $gcovFs"
    }
    $gcov = To-CMakePath $gcovFs

    Write-Host "==> Generating coverage report"
    $py = Join-Path $venv "Scripts\\python.exe"
    $gcovrArgs = @(
        "-m", "gcovr",
        "-r", (To-CMakePath (Join-Path $root "cpp\\ui-xp2p")),
        "--object-directory", (To-CMakePath $build),
        "--gcov-executable=$gcov",
        "--xml-pretty",
        "-o", (To-CMakePath (Join-Path $coverageDir "coverage.xml")),
        "--html-details", (To-CMakePath (Join-Path $coverageDir "index.html")),
        "--print-summary"
    )
    if ($env:XP2P_UI_COVER_DEBUG -eq "1") {
        Write-Host "==> gcovr: $py $($gcovrArgs -join ' ')"
    }
    & $py @gcovrArgs
}
