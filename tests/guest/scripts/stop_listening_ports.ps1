param(
    [Parameter(Mandatory = $true)]
    [string] $PortsBase64
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

try {
    $decoded = [System.Text.Encoding]::UTF8.GetString(
        [System.Convert]::FromBase64String($PortsBase64)
    )
} catch {
    throw "Failed to decode ports payload. Error: $($_.Exception.Message)"
}

try {
    $ports = ConvertFrom-Json -InputObject $decoded -ErrorAction Stop
} catch {
    throw "Failed to parse ports payload. Error: $($_.Exception.Message)"
}

if (-not ($ports -is [System.Collections.IEnumerable])) {
    exit 0
}

$targets = @{}
foreach ($port in $ports) {
    $value = 0
    if ([int]::TryParse([string]$port, [ref]$value)) {
        if ($value -gt 0) {
            $targets[$value] = $true
        }
    }
}
if ($targets.Count -eq 0) {
    exit 0
}

$netTcpCmd = Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue
$netUdpCmd = Get-Command Get-NetUDPEndpoint -ErrorAction SilentlyContinue
if ($netTcpCmd -or $netUdpCmd) {
    foreach ($port in $targets.Keys) {
        if ($netTcpCmd) {
            $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
            foreach ($listener in $listeners) {
                if ($listener.OwningProcess -gt 0) {
                    try {
                        Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
                    } catch { }
                }
            }
        }
        if ($netUdpCmd) {
            $endpoints = Get-NetUDPEndpoint -LocalPort $port -ErrorAction SilentlyContinue
            foreach ($endpoint in $endpoints) {
                if ($endpoint.OwningProcess -gt 0) {
                    try {
                        Stop-Process -Id $endpoint.OwningProcess -Force -ErrorAction SilentlyContinue
                    } catch { }
                }
            }
        }
    }
    exit 0
}

$lines = netstat -ano -p tcp | Select-String -Pattern "LISTENING"
foreach ($match in $lines) {
    $line = $match.Line
    if (-not $line) {
        continue
    }
    $parts = $line -split "\s+"
    if ($parts.Length -lt 5) {
        continue
    }
    $local = $parts[1]
    $pid = $parts[-1]
    if ($local -match ":(\\d+)$") {
        $port = [int]$Matches[1]
        if ($targets.ContainsKey($port)) {
            try {
                Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            } catch { }
        }
    }
}
if ($targets.Count -gt 0) {
    $udpLines = netstat -ano -p udp
    foreach ($line in $udpLines) {
        if (-not $line) {
            continue
        }
        $parts = $line -split "\s+"
        if ($parts.Length -lt 4) {
            continue
        }
        $local = $parts[1]
        $pid = $parts[-1]
        if ($local -match ":(\\d+)$") {
            $port = [int]$Matches[1]
            if ($targets.ContainsKey($port)) {
                try {
                    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
                } catch { }
            }
        }
    }
}
exit 0
