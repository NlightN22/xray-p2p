param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [switch]$ReplaceTag
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
    [System.IO.File]::WriteAllText($Path, $Content, $encoding)
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

Write-Section "Running go test ./..."
go test ./...

Write-Section "Running make lint"
make lint

Write-Section "Running make build"
make build

$OpenWrtMakefile = Join-Path -Path (Get-Location) -ChildPath "openwrt/feed/packages/utils/xp2p/Makefile"
if (-not (Test-Path $OpenWrtMakefile)) {
    Write-Error "OpenWrt package Makefile not found at $OpenWrtMakefile"
    exit 1
}

Write-Section "Updating OpenWrt package version"
$pkgPattern = '(?m)^(PKG_VERSION:=)(.*)$'
$releasePattern = '(?m)^(PKG_RELEASE:=)(.*)$'
$pkgContent = Get-Content -Raw $OpenWrtMakefile
$pkgUpdated = $pkgContent -replace $pkgPattern, { "$($matches[1])$Version" }
$pkgUpdated = $pkgUpdated -replace $releasePattern, { "$($matches[1])1" }
if ($pkgContent -eq $pkgUpdated) {
    Write-Error "Failed to update PKG_VERSION in $OpenWrtMakefile"
    exit 1
}
Write-Utf8NoBom -Path $OpenWrtMakefile -Content $pkgUpdated

$pending = git status --porcelain
if (-not $pending) {
    Write-Error "No changes detected after version bump; aborting."
    exit 1
}

Write-Section "Creating release commit"
git commit -am "chore: release $Tag"

Write-Section "Tagging $Tag"
git tag $Tag

Write-Section "Pushing branch main"
git push origin main

Write-Section "Pushing tag $Tag"
git push origin $Tag

Write-Section "Release $Tag complete"
git status -s
