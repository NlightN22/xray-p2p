param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$ErrorActionPreference = 'Stop'

function Write-Section {
    param([string]$Message)
    Write-Host "`n=== $Message ===" -ForegroundColor Cyan
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
if ((git rev-parse -q --verify "$Tag" 2>$null)) {
    Write-Error "Tag $Tag already exists locally"
    exit 1
}
if ((git ls-remote --exit-code origin "refs/tags/$Tag" 2>$null)) {
    Write-Error "Tag $Tag already exists on origin"
    exit 1
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
if ($original -eq $updated) {
    Write-Error "Version placeholder not found in $VersionFile"
    exit 1
}
[System.IO.File]::WriteAllText(
    $VersionFile,
    $updated,
    [System.Text.Encoding]::UTF8
)

Write-Section "Running go test ./..."
go test ./...

Write-Section "Running make build"
make build

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
