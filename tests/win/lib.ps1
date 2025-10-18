Set-StrictMode -Version Latest

if (-not (Get-Variable -Name LibraryRoot -Scope Script -ErrorAction SilentlyContinue)) {
    $script:LibraryRoot = Split-Path -Parent $PSCommandPath
}

if (-not (Get-Variable -Name XrayWinState -Scope Script -ErrorAction SilentlyContinue)) {
    $script:XrayWinState = @{
        VerboseLogs = $false
        TestResults = [System.Collections.ArrayList]::new()
        TestContext = $null
    }
}

function Get-LibraryRoot {
    return $script:LibraryRoot
}

function Get-StateValue {
    param([Parameter(Mandatory=$true)][string]$Name)
    $state = $script:XrayWinState
    if (-not $state.ContainsKey($Name)) {
        return $null
    }
    return $state[$Name]
}

function Set-StateValue {
    param(
        [Parameter(Mandatory=$true)][string]$Name,
        $Value
    )
    $script:XrayWinState[$Name] = $Value
}

$moduleRoot = Join-Path (Get-LibraryRoot) 'lib'

$moduleFiles = @(
    'state.ps1',
    'context.ps1',
    'vagrant.ps1',
    'xray.ps1'
)

foreach ($moduleFile in $moduleFiles) {
    $fullPath = Join-Path $moduleRoot $moduleFile
    if (-not (Test-Path -LiteralPath $fullPath)) {
        throw "Module file not found: $fullPath"
    }
    . $fullPath
}
