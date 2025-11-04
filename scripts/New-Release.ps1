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
$pattern = 'const current = ".*"'
$replacement = "const current = `"$Version`""
(Get-Content $VersionFile) `
    -replace $pattern, $replacement `
    | Set-Content -NoNewline $VersionFile

Write-Section "Running go test ./..."
go test ./...

Write-Section "Running make build"
make build

Write-Section "Version update complete"
git status -s
Write-Host "`nRun the following to finalize the release:" -ForegroundColor Yellow
Write-Host "  git commit -am `"chore: release $Tag`""
Write-Host "  git tag $Tag"
Write-Host "  git push origin main"
Write-Host "  git push origin $Tag"
