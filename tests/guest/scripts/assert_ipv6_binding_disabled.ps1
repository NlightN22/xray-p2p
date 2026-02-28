param(
    [Parameter(Mandatory = $true)]
    [string]$InterfaceName,
    [int]$TimeoutSeconds = 20,
    [int]$PollSeconds = 1
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($InterfaceName)) {
    throw "InterfaceName is required"
}

function Wait-ForNetAdapterBinding {
    param(
        [string]$Name,
        [int]$Timeout,
        [int]$Poll
    )

    $deadline = (Get-Date).AddSeconds($Timeout)
    do {
        $binding = Get-NetAdapterBinding -Name $Name -ComponentID "ms_tcpip6" -ErrorAction SilentlyContinue |
            Select-Object -First 1
        if ($binding) {
            if (-not $binding.Enabled) {
                return
            }
        }
        Start-Sleep -Seconds $Poll
    } while ((Get-Date) -lt $deadline)

    if ($binding) {
        throw "IPv6 binding is still enabled on $Name"
    }
    throw "Adapter $Name not found"
}

function Wait-ForNetshDisabled {
    param(
        [string]$Name,
        [int]$Timeout,
        [int]$Poll
    )

    $escaped = [regex]::Escape($Name)
    $deadline = (Get-Date).AddSeconds($Timeout)
    $lastLine = $null
    do {
        $lines = & netsh interface ipv6 show interface
        foreach ($line in $lines) {
            if ($line -match $escaped) {
                $lastLine = $line
                if ($line -match "\bdisabled\b") {
                    return
                }
            }
        }
        Start-Sleep -Seconds $Poll
    } while ((Get-Date) -lt $deadline)

    if ($lastLine) {
        throw "IPv6 interface not disabled: $lastLine"
    }
    throw "Adapter $Name not found"
}

$cmd = Get-Command Get-NetAdapterBinding -ErrorAction SilentlyContinue
if ($cmd) {
    Wait-ForNetAdapterBinding -Name $InterfaceName -Timeout $TimeoutSeconds -Poll $PollSeconds
    exit 0
}

Wait-ForNetshDisabled -Name $InterfaceName -Timeout $TimeoutSeconds -Poll $PollSeconds
exit 0
