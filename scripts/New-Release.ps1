param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [switch]$ReplaceTag,
    [switch]$Quiet,
    [switch]$SkipOpenWrtArtifacts,
    [string]$ArtifactsBranch = "artifacts",
    [string]$ArtifactsDir = "openwrt/staging/stable"
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

function Confirm-StepDefaultYes {
    param([string]$Message)
    if ($Quiet) {
        return $true
    }
    $answer = Read-Host "$Message [Y/n]"
    if ($null -eq $answer) {
        return $true
    }
    $trimmed = $answer.Trim()
    if ($trimmed.Length -eq 0) {
        return $true
    }
    return $trimmed -match '^(?i)y(es)?$'
}

function Assert-CleanWorkingTree {
    $pending = git status --porcelain
    if ($pending -ne $null -and $pending.Trim().Length -gt 0) {
        Write-Error "Working tree is not clean. Commit or stash changes first."
        exit 1
    }
}

function Test-GitLocalBranch {
    param([Parameter(Mandatory = $true)][string]$Name)
    git show-ref --verify --quiet "refs/heads/$Name" 2>$null
    return $LASTEXITCODE -eq 0
}

function Test-GitRemoteBranch {
    param([Parameter(Mandatory = $true)][string]$Name)
    git ls-remote --exit-code origin "refs/heads/$Name" 2>$null | Out-Null
    return $LASTEXITCODE -eq 0
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

if (-not $SkipOpenWrtArtifacts) {
    Write-Section "Building OpenWrt .ipk into $ArtifactsDir"
    $doBuild = Confirm-StepDefaultYes "Build OpenWrt .ipk (Vagrant) and stage into $ArtifactsDir"
    if ($doBuild) {
        $vagrantDir = Join-Path -Path (Get-Location) -ChildPath "infra/vagrant/debian12/ipk-build"
        if (-not (Test-Path $vagrantDir)) {
            Write-Error "Vagrant directory not found at $vagrantDir"
            exit 1
        }

        $artifactsDirClean = $ArtifactsDir.Trim().TrimStart('\', '/')
        if ([string]::IsNullOrWhiteSpace($artifactsDirClean)) {
            Write-Error "ArtifactsDir is empty"
            exit 1
        }

        $hostArtifactsDir = Join-Path -Path (Get-Location) -ChildPath $artifactsDirClean
        New-Item -ItemType Directory -Force -Path $hostArtifactsDir | Out-Null

        Push-Location $vagrantDir
        try {
            vagrant up
            $guestDir = "/srv/xray-p2p/$($artifactsDirClean -replace '\\', '/')"
            $cmd = "/srv/xray-p2p/scripts/build/build_openwrt_ipk.sh --all --force-build --output-dir $guestDir"
            vagrant ssh -c $cmd
        } finally {
            Pop-Location
        }

        $built = Get-ChildItem -Path $hostArtifactsDir -Filter "*.ipk" -File -ErrorAction SilentlyContinue
        if (-not $built -or $built.Count -eq 0) {
            Write-Error "No .ipk files found under $hostArtifactsDir after build"
            exit 1
        }
        Write-Host ("Built {0} .ipk files into {1}" -f $built.Count, $hostArtifactsDir)
    } else {
        Write-Host "Skipped OpenWrt .ipk build/stage" -ForegroundColor Yellow
    }
}

Write-Section "OpenWrt feed packaging note"
Write-Host "OpenWrt .ipk files are not committed to main." -ForegroundColor Yellow
Write-Host "Build them locally and push to the dedicated artifacts branch under openwrt/staging/stable/." -ForegroundColor Yellow

$pending = git status --porcelain --untracked-files=no
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

if (-not $SkipOpenWrtArtifacts) {
    Write-Section "Publishing OpenWrt .ipk to branch $ArtifactsBranch"
    if (Confirm-StepDefaultYes "Commit and push $ArtifactsDir/*.ipk to branch $ArtifactsBranch") {
        $artifactsDirClean = $ArtifactsDir.Trim().TrimStart('\', '/')
        $hostArtifactsDir = Join-Path -Path (Get-Location) -ChildPath $artifactsDirClean

        $ipks = Get-ChildItem -Path $hostArtifactsDir -Filter "*.ipk" -File -ErrorAction SilentlyContinue
        if (-not $ipks -or $ipks.Count -eq 0) {
            Write-Error "No .ipk files found under $hostArtifactsDir"
            exit 1
        }

        $hasRemote = Test-GitRemoteBranch -Name $ArtifactsBranch
        $worktreeRoot = Join-Path -Path (Get-Location) -ChildPath ".tmp/artifacts-worktree"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $worktreeRoot) | Out-Null
        if (Test-Path $worktreeRoot) {
            git worktree remove --force $worktreeRoot 2>$null | Out-Null
            Remove-Item -Recurse -Force $worktreeRoot -ErrorAction SilentlyContinue
        }
        if ($hasRemote) {
            git fetch origin $ArtifactsBranch | Out-Null
            git worktree add -B $ArtifactsBranch $worktreeRoot "origin/$ArtifactsBranch"
        } else {
            git worktree add -B $ArtifactsBranch $worktreeRoot
        }

        $destArtifactsDir = Join-Path -Path $worktreeRoot -ChildPath $artifactsDirClean
        New-Item -ItemType Directory -Force -Path $destArtifactsDir | Out-Null
        Get-ChildItem -Path $destArtifactsDir -Filter "*.ipk" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
        Copy-Item -Force -Path $ipks.FullName -Destination $destArtifactsDir

        git -C $worktreeRoot add -- "$artifactsDirClean"
        $message = "chore(openwrt): stage ipk for $Tag"
        $staged = git -C $worktreeRoot diff --cached --name-only
        if ($staged -ne $null -and $staged.Trim().Length -gt 0) {
            git -C $worktreeRoot commit -m $message
        } else {
            Write-Host "No changes to commit on $ArtifactsBranch" -ForegroundColor Yellow
        }

        if (Confirm-Push "branch $ArtifactsBranch to origin") {
            git -C $worktreeRoot push -u origin $ArtifactsBranch
        } else {
            Write-Host "Skipping push of branch $ArtifactsBranch" -ForegroundColor Yellow
        }
        git worktree remove --force $worktreeRoot | Out-Null
        Remove-Item -Recurse -Force $worktreeRoot -ErrorAction SilentlyContinue
    } else {
        Write-Host "Skipped artifacts branch publish" -ForegroundColor Yellow
    }
}

Write-Section "Release $Tag complete"
git status -s
