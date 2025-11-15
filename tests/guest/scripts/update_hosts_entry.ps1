param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("Add", "Remove")]
    [string]$Action,

    [Parameter(Mandatory = $true)]
    [string]$HostName,

    [string]$IPAddress
)

$ErrorActionPreference = 'Stop'
$hostsPath = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
if (-not (Test-Path $hostsPath)) {
    throw "Hosts file not found at $hostsPath"
}

function Remove-HostEntry {
    param(
        [string[]]$Lines,
        [string]$Host
    )
    $target = $Host.ToLower()
    $filtered = @()
    foreach ($line in $Lines) {
        $trimmed = $line.Trim()
        if (-not $trimmed -or $trimmed.StartsWith('#')) {
            $filtered += $line
            continue
        }
        $parts = $trimmed -split '[\s\t]+' | Where-Object { $_ -ne '' }
        if ($parts.Length -lt 2) {
            $filtered += $line
            continue
        }
        $skip = $false
        foreach ($part in $parts[1..($parts.Length - 1)]) {
            if ($part.StartsWith('#')) {
                break
            }
            if ($part.ToLower() -eq $target) {
                $skip = $true
                break
            }
        }
        if (-not $skip) {
            $filtered += $line
        }
    }
    return ,$filtered
}

$existing = Get-Content -Path $hostsPath -ErrorAction Stop
$filtered = Remove-HostEntry -Lines $existing -Host $HostName

if ($Action -eq 'Add') {
    if (-not $IPAddress) {
        throw "IPAddress is required for Add action"
    }
    $filtered += "$IPAddress $HostName"
}

Set-Content -Path $hostsPath -Value $filtered -Encoding ASCII
