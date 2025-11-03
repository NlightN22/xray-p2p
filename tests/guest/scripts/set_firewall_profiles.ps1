param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Enable', 'Disable')]
    [string]$State,

    [Parameter()]
    [string]$Profiles = 'Domain,Private,Public'
)

$ErrorActionPreference = 'Stop'

try {
    $profileList = $Profiles -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    if (-not $profileList) {
        throw "No firewall profiles specified."
    }

    $enableValue = if ($State -eq 'Enable') { 'True' } else { 'False' }
    foreach ($profile in $profileList) {
        Set-NetFirewallProfile -Profile $profile -Enabled $enableValue -ErrorAction Stop
    }
    exit 0
}
catch {
    Write-Error $_
    exit 1
}
