param(
    [Parameter(Mandatory = $true)]
    [string]$Xp2pPath,
    [Parameter(Mandatory = $true)]
    [string]$Role,
    [Parameter(Mandatory = $true)]
    [string]$InstallRoot,
    [Parameter(Mandatory = $true)]
    [string]$ConfigDir,
    [string]$LogPathsBase64 = '',
    [string]$RemoveConfig = 'false'
)

$ErrorActionPreference = 'Stop'

$removeConfigFlag = $false
if ($RemoveConfig) {
    if ($RemoveConfig -is [string]) {
        $removeConfigFlag = ($RemoveConfig.ToLower() -eq 'true')
    } else {
        $removeConfigFlag = [bool]$RemoveConfig
    }
}

$paths = @()
if ($LogPathsBase64) {
    $raw = [System.Text.Encoding]::UTF8.GetString(
        [System.Convert]::FromBase64String($LogPathsBase64)
    )
    $paths = $raw | ConvertFrom-Json
}

if (Test-Path $Xp2pPath) {
    try {
        & $Xp2pPath $Role service stop | Out-Null
    } catch {
        # Cleanup continues even if service stop fails.
    }

    if ($removeConfigFlag) {
        try {
            if ($Role -eq 'client') {
                & $Xp2pPath $Role remove --path $InstallRoot --config-dir $ConfigDir --quiet --ignore-missing --all | Out-Null
            } else {
                & $Xp2pPath $Role remove --path $InstallRoot --config-dir $ConfigDir --quiet --ignore-missing | Out-Null
            }
        } catch {
            # Cleanup continues even if remove fails.
        }
    }
}

foreach ($path in $paths) {
    if (Test-Path $path) {
        Remove-Item $path -Force -Recurse -ErrorAction SilentlyContinue
    }
}

exit 0
