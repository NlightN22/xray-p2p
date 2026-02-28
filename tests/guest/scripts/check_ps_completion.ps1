param(
    [Parameter(Mandatory = $true)]
    [string] $CompletionPath,
    [string] $InstallRoot = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Assert-ProfileLine {
    param(
        [Parameter(Mandatory = $true)]
        [string] $ProfilePath,
        [Parameter(Mandatory = $true)]
        [string] $ExpectedLine,
        [Parameter(Mandatory = $true)]
        [string] $Label
    )

    if (-not (Test-Path -LiteralPath $ProfilePath)) {
        Write-Output "__MISSING_PROFILE__:${Label}:$ProfilePath"
        exit 3
    }

    $content = Get-Content -LiteralPath $ProfilePath -ErrorAction SilentlyContinue
    if ($content -notcontains $ExpectedLine) {
        Write-Output "__MISSING_LINE__:${Label}:$ProfilePath"
        exit 4
    }
}

function Assert-PathEntry {
    param([string] $PathEntry)
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return
    }
    $current = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    if (-not $current) {
        Write-Output "__MISSING_PATH__:$PathEntry"
        exit 5
    }
    $expected = $PathEntry.Trim().Trim('"').TrimEnd('\')
    $segments = $current -split ';' | ForEach-Object { $_.Trim().Trim('\"') } | Where-Object { $_ }
    $match = $false
    foreach ($segment in $segments) {
        if ($segment.Trim('\"').TrimEnd('\').Equals($expected, [System.StringComparison]::InvariantCultureIgnoreCase)) {
            $match = $true
            break
        }
    }
    if (-not $match) {
        Write-Output "__MISSING_PATH__:$PathEntry"
        exit 5
    }
}

function Assert-NoProfileErrors {
    param(
        [Parameter(Mandatory = $true)]
        [string] $ShellExe,
        [Parameter(Mandatory = $true)]
        [string] $CompletionPath
    )

    $stderrFile = [System.IO.Path]::GetTempFileName()
    try {
        & $ShellExe -NoLogo -NonInteractive -Command '$null' 2> $stderrFile | Out-Null
        $stderrText = Get-Content -LiteralPath $stderrFile -Raw
        if ($stderrText -match [regex]::Escape($CompletionPath)) {
            Write-Output "__PROFILE_ERROR__:${ShellExe}:$CompletionPath"
            exit 7
        }
        if ($stderrText -match 'CommandNotFoundException|ObjectNotFound') {
            Write-Output "__PROFILE_ERROR__:${ShellExe}:startup"
            exit 7
        }
    }
    finally {
        Remove-Item -LiteralPath $stderrFile -Force -ErrorAction SilentlyContinue
    }
}

if (-not (Test-Path -LiteralPath $CompletionPath)) {
    Write-Output "__MISSING_COMPLETION__:$CompletionPath"
    exit 2
}

$resolvedCompletion = (Resolve-Path -LiteralPath $CompletionPath).Path
$expectedLine = "if (Test-Path '$resolvedCompletion') { . '$resolvedCompletion' }"

Assert-PathEntry -PathEntry $InstallRoot
Assert-ProfileLine -ProfilePath $PROFILE.AllUsersAllHosts -ExpectedLine $expectedLine -Label "windows-powershell"
Assert-NoProfileErrors -ShellExe (Get-Command -Name powershell.exe).Source -CompletionPath $resolvedCompletion

$pwsh = Get-Command -Name pwsh.exe -ErrorAction SilentlyContinue
if ($pwsh) {
    try {
        $pwshProfile = & $pwsh.Source -NoProfile -NonInteractive -Command '$PROFILE.AllUsersAllHosts' 2>$null
        $pwshProfile = $pwshProfile.Trim()
        if (-not [string]::IsNullOrWhiteSpace($pwshProfile)) {
            Assert-ProfileLine -ProfilePath $pwshProfile -ExpectedLine $expectedLine -Label "pwsh"
        }
        Assert-NoProfileErrors -ShellExe $pwsh.Source -CompletionPath $resolvedCompletion
    }
    catch {
        Write-Output "__PWSH_QUERY_FAILED__:$($_.Exception.Message)"
        exit 6
    }
}

exit 0
