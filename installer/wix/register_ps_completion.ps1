param(
    [Parameter(Mandatory = $true)]
    [string] $CompletionPath,
    [string] $InstallRoot = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Ensure-ProfileLine {
    param(
        [Parameter(Mandatory = $true)]
        [string] $ProfilePath,
        [Parameter(Mandatory = $true)]
        [string] $Line,
        [string] $LegacyPath = ''
    )

    $profileDir = Split-Path -Path $ProfilePath -Parent
    if (-not (Test-Path -LiteralPath $profileDir)) {
        New-Item -ItemType Directory -Path $profileDir -Force | Out-Null
    }
    if (-not (Test-Path -LiteralPath $ProfilePath)) {
        New-Item -ItemType File -Path $ProfilePath -Force | Out-Null
    }

    $existing = Get-Content -LiteralPath $ProfilePath -ErrorAction SilentlyContinue
    $updated = @($existing)
    if ($LegacyPath) {
        $escaped = [regex]::Escape($LegacyPath)
        $legacyPattern = "^\s*\.\s+['""]$escaped['""]\s*$"
        $updated = $updated | Where-Object { $_ -notmatch $legacyPattern }
    }
    if ($updated -contains $Line) {
        return
    }
    if ($updated.Count -ne $existing.Count) {
        Set-Content -LiteralPath $ProfilePath -Value $updated
    }
    Add-Content -LiteralPath $ProfilePath -Value $Line
}

function Ensure-PathEntry {
    param([string] $PathEntry)
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return
    }
    $normalizedEntry = $PathEntry.Trim().Trim('"').TrimEnd('\')
    $current = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $segments = @()
    if ($current) {
        $segments = $current -split ';' | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    }
    $found = $false
    $updatedSegments = @()
    foreach ($segment in $segments) {
        $normalizedSegment = $segment.Trim('"').TrimEnd('\')
        if ($normalizedSegment.Equals($normalizedEntry, [System.StringComparison]::InvariantCultureIgnoreCase)) {
            $found = $true
            $updatedSegments += $normalizedEntry
            continue
        }
        $updatedSegments += $segment.Trim('"')
    }
    if (-not $found) {
        $updatedSegments = @($normalizedEntry) + $updatedSegments
    }
    $newPath = ($updatedSegments -join ';')
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
    $env:Path = $newPath
}

if (-not (Test-Path -LiteralPath $CompletionPath)) {
    throw "Completion script not found at $CompletionPath"
}

$resolvedCompletion = (Resolve-Path -LiteralPath $CompletionPath).Path
$completionLine = "if (Test-Path '$resolvedCompletion') { . '$resolvedCompletion' }"
$legacyCompletion = ". '$resolvedCompletion'"

Ensure-PathEntry -PathEntry $InstallRoot
$winPsProfile = Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\profile.ps1'
Ensure-ProfileLine -ProfilePath $winPsProfile -Line $completionLine -LegacyPath $legacyCompletion
$winPsWow64Dir = Join-Path $env:WINDIR 'SysWOW64\WindowsPowerShell\v1.0'
if (Test-Path -LiteralPath $winPsWow64Dir) {
    $winPsWow64Profile = Join-Path $winPsWow64Dir 'profile.ps1'
    Ensure-ProfileLine -ProfilePath $winPsWow64Profile -Line $completionLine -LegacyPath $legacyCompletion
}

$pwsh = Get-Command -Name pwsh.exe -ErrorAction SilentlyContinue
if ($pwsh) {
    try {
        $pwshProfile = & $pwsh.Source -NoProfile -NonInteractive -Command '$PROFILE.AllUsersAllHosts' 2>$null
        $pwshProfile = $pwshProfile.Trim()
        if (-not [string]::IsNullOrWhiteSpace($pwshProfile)) {
            Ensure-ProfileLine -ProfilePath $pwshProfile -Line $completionLine -LegacyPath $legacyCompletion
        }
    }
    catch {
        Write-Host "Skipping pwsh profile update: $($_.Exception.Message)"
    }
}
