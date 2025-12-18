param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [switch]$ReplaceTag,
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'

function Write-Section {
    param([string]$Message)
    Write-Host "`n=== $Message ===" -ForegroundColor Cyan
}

function Write-Utf8NoBom {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )
    $encoding = New-Object System.Text.UTF8Encoding $false
    $normalized = $Content -replace "`r`n", "`n" -replace "`r", "`n"
    [System.IO.File]::WriteAllText($Path, $normalized, $encoding)
}

function Confirm-Push {
    param([string]$Target)
    if ($Quiet) {
        return $true
    }
    $answer = Read-Host "Push $Target? [y/N]"
    return $answer -match '^(?i)y(es)?$'
}

function Confirm-Step {
    param([string]$Message)
    if ($Quiet) {
        return $true
    }
    $answer = Read-Host "$Message [y/N]"
    return $answer -match '^(?i)y(es)?$'
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Error "git is required"
    exit 1
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "go is required"
    exit 1
}

$Version = $Version.Trim()
if ($Version -notmatch '^\d+(\.\d+){1,2}$') {
    Write-Error "Version should be a semantic version without leading 'v' (e.g. 0.2.0)"
    exit 1
}

$Tag = "v$Version"

Write-Section "Checking for existing tag $Tag"
$localTagExists = $null -ne (git rev-parse -q --verify "$Tag" 2>$null)
$remoteTagExists = $null -ne (git ls-remote --exit-code origin "refs/tags/$Tag" 2>$null)
if (-not $ReplaceTag) {
    if ($localTagExists) {
        Write-Error "Tag $Tag already exists locally (use -ReplaceTag to recreate it)"
        exit 1
    }
    if ($remoteTagExists) {
        Write-Error "Tag $Tag already exists on origin (use -ReplaceTag to recreate it)"
        exit 1
    }
} else {
    Write-Section "Replacing existing tag $Tag"
    if ($localTagExists) {
        git tag -d "$Tag" | Out-Null
    }
    if ($remoteTagExists) {
        git push --delete origin "$Tag" | Out-Null
    }
}

$VersionFile = Join-Path -Path (Get-Location) -ChildPath "go/internal/version/version.go"
if (-not (Test-Path $VersionFile)) {
    Write-Error "Version file not found at $VersionFile"
    exit 1
}

Write-Section "Updating version file"
$pattern = 'var current = ".*"'
$replacement = "var current = `"$Version`""
$original = Get-Content -Raw $VersionFile
$updated = $original -replace $pattern, $replacement
if ($original -notmatch $pattern) {
    Write-Error "Version placeholder not found in $VersionFile"
    exit 1
}
if ($original -ne $updated) {
    [System.IO.File]::WriteAllText(
        $VersionFile,
        $updated,
        [System.Text.Encoding]::UTF8
    )
    & gofmt -w $VersionFile
} else {
    Write-Section "Version file already set to $Version"
}

Write-Section "Running make build-deb"
if (Confirm-Step "Run make build-deb") {
    make build-deb
} else {
    Write-Host "Skipped make build-deb" -ForegroundColor Yellow
}



$OpenWrtMakefile = Join-Path -Path (Get-Location) -ChildPath "openwrt/feed/packages/utils/xp2p/Makefile"
if (-not (Test-Path $OpenWrtMakefile)) {
    Write-Error "OpenWrt package Makefile not found at $OpenWrtMakefile"
    exit 1
}

Write-Section "Updating OpenWrt package version"
$pkgPattern = '(?m)^(PKG_VERSION:=)(.*)$'
$releasePattern = '(?m)^(PKG_RELEASE:=)(.*)$'
$pkgContent = Get-Content -Raw $OpenWrtMakefile
$pkgUpdated = $pkgContent -replace $pkgPattern, "`${1}$Version"
$pkgUpdated = $pkgUpdated -replace $releasePattern, "`${1}1"
if ($pkgContent -eq $pkgUpdated) {
    Write-Section "OpenWrt package version already set to $Version"
} else {
    Write-Utf8NoBom -Path $OpenWrtMakefile -Content $pkgUpdated
}

Write-Section "Running make build-ipk"
if (Confirm-Step "Run make build-ipk") {
    make build-ipk
} else {
    Write-Host "Skipped make build-ipk" -ForegroundColor Yellow
}

Write-Section "Staging OpenWrt repository artifacts"
$repoRoot = Join-Path -Path (Get-Location) -ChildPath "openwrt/repo"
if (Test-Path $repoRoot) {
    $ipks = Get-ChildItem -Path $repoRoot -Recurse -Filter "*.ipk" -ErrorAction SilentlyContinue
    if ($ipks) {
        git add -- $repoRoot
        Write-Host ("Staged {0} IPK files from {1}" -f $ipks.Count, $repoRoot)
    } else {
        Write-Host "No IPK files found under $repoRoot" -ForegroundColor Yellow
    }
} else {
    Write-Host "OpenWrt repo directory $repoRoot not found; skipping staging" -ForegroundColor Yellow
}

$pending = git status --porcelain
$changesPresent = $pending -ne $null -and $pending.Trim().Length -gt 0
if ($changesPresent) {
    Write-Section "Creating release commit"
    git commit -am "chore: release $Tag"
} else {
    Write-Section "No changes to commit; reusing current HEAD for tagging"
}

Write-Section "Tagging $Tag"
git tag $Tag

Write-Section "Pushing branch main"
if (Confirm-Push "branch main to origin") {
    git push origin main
} else {
    Write-Host "Skipping push of branch main" -ForegroundColor Yellow
}

Write-Section "Pushing tag $Tag"
if (Confirm-Push "tag $Tag to origin") {
    git push origin $Tag
} else {
    Write-Host "Skipping push of tag $Tag" -ForegroundColor Yellow
}

Write-Section "Release $Tag complete"
git status -s
