param(
    [Parameter(Mandatory = $true)]
    [string] $OutputPath,

    [string] $Label = ""
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$dir = Split-Path -Parent $OutputPath
if ($dir -and -not (Test-Path $dir)) {
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}

$header = "=== NET STATE " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss") + " ==="
if ($Label) {
    $header += " [" + $Label + "]"
}

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add($header)
$lines.Add("")
$lines.Add("Adapters:")
try {
    $adapters = Get-NetAdapter -IncludeHidden | Select-Object Name,InterfaceAlias,Status,InterfaceDescription,ifIndex
    foreach ($item in $adapters) {
        $lines.Add(("{0} | {1} | {2} | {3} | {4}" -f $item.Name, $item.InterfaceAlias, $item.Status, $item.InterfaceDescription, $item.ifIndex))
    }
} catch {
    $lines.Add("ERROR: Get-NetAdapter failed: " + $_.Exception.Message)
}

$lines.Add("")
$lines.Add("IPv4 Interfaces:")
try {
    $ipif = Get-NetIPInterface -AddressFamily IPv4 | Select-Object InterfaceAlias,InterfaceIndex,ConnectionState,NlMtu,InterfaceMetric
    foreach ($item in $ipif) {
        $lines.Add(("{0} | {1} | {2} | {3} | {4}" -f $item.InterfaceAlias, $item.InterfaceIndex, $item.ConnectionState, $item.NlMtu, $item.InterfaceMetric))
    }
} catch {
    $lines.Add("ERROR: Get-NetIPInterface failed: " + $_.Exception.Message)
}

$lines.Add("")
$lines.Add("Processes (xp2p/xray):")
try {
    $procs = Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Select-Object Name,Id,Path,StartTime
    if (-not $procs) {
        $lines.Add("(none)")
    } else {
        foreach ($item in $procs) {
            $lines.Add(("{0} | {1} | {2} | {3}" -f $item.Name, $item.Id, $item.Path, $item.StartTime))
        }
    }
} catch {
    $lines.Add("ERROR: Get-Process failed: " + $_.Exception.Message)
}

$lines.Add("")
$lines.Add("Services (xp2p-client/xp2p-server):")
try {
    $services = Get-Service -Name xp2p-client,xp2p-server -ErrorAction SilentlyContinue | Select-Object Name,Status,StartType
    if (-not $services) {
        $lines.Add("(none)")
    } else {
        foreach ($item in $services) {
            $lines.Add(("{0} | {1} | {2}" -f $item.Name, $item.Status, $item.StartType))
        }
    }
} catch {
    $lines.Add("ERROR: Get-Service failed: " + $_.Exception.Message)
}

$lines.Add("")
[System.IO.File]::AppendAllLines($OutputPath, $lines, [System.Text.Encoding]::ASCII)
