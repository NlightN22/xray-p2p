param(
    [Parameter(Mandatory = $true)]
    [string[]] $ServiceName,
    [Parameter(Mandatory = $true)]
    [string] $Sid
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($ServiceName.Count -eq 1 -and $ServiceName[0] -like "*,*") {
    $ServiceName = $ServiceName[0].Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ }
}

function Get-ServiceSddl {
    param([string] $Name)

    $output = & sc.exe sdshow $Name 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "sc.exe sdshow failed for ${Name}: $output"
    }

    $joined = ($output -join "`n").Trim()
    $match = [regex]::Match($joined, "O:.*")
    if (-not $match.Success) {
        $match = [regex]::Match($joined, "D:.*")
    }
    if (-not $match.Success) {
        throw "Service SDDL not found for $Name."
    }
    return $match.Value.Trim()
}

function Add-AceToSddl {
    param(
        [string] $Sddl,
        [string] $Ace
    )

    if ($Sddl -match [regex]::Escape($Ace)) {
        return $Sddl
    }
    $saclIndex = $Sddl.IndexOf("S:")
    if ($saclIndex -gt 0) {
        return $Sddl.Insert($saclIndex, $Ace)
    }
    return $Sddl + $Ace
}

$ace = "(A;;LCRPWP;;;$Sid)"
foreach ($name in $ServiceName) {
    $current = Get-ServiceSddl -Name $name
    if ($current -match [regex]::Escape($ace)) {
        continue
    }

    $updated = Add-AceToSddl -Sddl $current -Ace $ace
    $result = & sc.exe sdset $name $updated 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "sc.exe sdset failed for ${name}: $result"
    }
}
