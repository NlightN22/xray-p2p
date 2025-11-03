param(
    [Parameter(Mandatory = $true)]
    [string]$Name,

    [Parameter(Mandatory = $true)]
    [string]$RemoteAddress,

    [Parameter(Mandatory = $true)]
    [int]$LocalPort,

    [Parameter()]
    [ValidateSet('TCP', 'UDP')]
    [string]$Protocol = 'TCP',

    [Parameter()]
    [ValidateSet('Present', 'Absent')]
    [string]$Ensure = 'Present'
)

$ErrorActionPreference = 'Stop'

try {
    if ($Ensure -eq 'Present') {
        Get-NetFirewallRule -DisplayName $Name -ErrorAction SilentlyContinue | ForEach-Object {
            Remove-NetFirewallRule -DisplayName $Name -ErrorAction SilentlyContinue
        }
        New-NetFirewallRule \
            -DisplayName $Name \
            -Direction Inbound \
            -Action Block \
            -Protocol $Protocol \
            -LocalPort $LocalPort \
            -RemoteAddress $RemoteAddress \
            -Profile Any \
            -EdgeTraversalPolicy Block | Out-Null
    }
    else {
        Get-NetFirewallRule -DisplayName $Name -ErrorAction SilentlyContinue | ForEach-Object {
            Remove-NetFirewallRule -DisplayName $Name -ErrorAction SilentlyContinue
        }
    }
    exit 0
}
catch {
    Write-Error $_
    exit 1
}
