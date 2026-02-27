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
        [string] $Line
    )

    $profileDir = Split-Path -Path $ProfilePath -Parent
    if (-not (Test-Path -LiteralPath $profileDir)) {
        New-Item -ItemType Directory -Path $profileDir -Force | Out-Null
    }
    if (-not (Test-Path -LiteralPath $ProfilePath)) {
        New-Item -ItemType File -Path $ProfilePath -Force | Out-Null
    }

    $existing = Get-Content -LiteralPath $ProfilePath -ErrorAction SilentlyContinue
    if ($existing -contains $Line) {
        return
    }

    Add-Content -LiteralPath $ProfilePath -Value $Line
}

function Ensure-PathEntry {
    param([string] $PathEntry)
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return
    }
    $current = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $segments = $current -split ';'
    if ($segments -contains $PathEntry) {
        return
    }
    $newPath = "$PathEntry;$current"
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
    $env:Path = "$PathEntry;$env:Path"
}

if (-not (Test-Path -LiteralPath $CompletionPath)) {
    throw "Completion script not found at $CompletionPath"
}

$resolvedCompletion = (Resolve-Path -LiteralPath $CompletionPath).Path
$completionLine = ". '$resolvedCompletion'"

Ensure-PathEntry -PathEntry $InstallRoot
Ensure-ProfileLine -ProfilePath $PROFILE.AllUsersAllHosts -Line $completionLine

$pwsh = Get-Command -Name pwsh.exe -ErrorAction SilentlyContinue
if ($pwsh) {
    try {
        $pwshProfile = & $pwsh.Source -NoProfile -NonInteractive -Command '$PROFILE.AllUsersAllHosts' 2>$null
        $pwshProfile = $pwshProfile.Trim()
        if (-not [string]::IsNullOrWhiteSpace($pwshProfile)) {
            Ensure-ProfileLine -ProfilePath $pwshProfile -Line $completionLine
        }
    }
    catch {
        Write-Host "Skipping pwsh profile update: $($_.Exception.Message)"
    }
}
