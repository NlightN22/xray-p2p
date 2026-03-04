param(
    [Parameter(Mandatory = $true)]
    [int] $Port,
    [int] $TimeoutSeconds = 30,
    [int] $PollSeconds = 1
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($Port -le 0) {
    throw "Port must be a positive integer."
}

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$netCmd = Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue

while ((Get-Date) -lt $deadline) {
    if ($netCmd) {
        $listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        if ($listeners) {
            exit 0
        }
    } else {
        $lines = & netstat -an -p tcp
        foreach ($line in $lines) {
            if ($line -match "[:.]$Port\\s+LISTENING\\b") {
                exit 0
            }
        }
    }
    Start-Sleep -Seconds $PollSeconds
}

Write-Output "Timed out waiting for TCP listener on port $Port."
exit 4
