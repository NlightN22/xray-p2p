param(
    [Parameter(Mandatory = $true)]
    [string] $NamesBase64
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$payload = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($NamesBase64))
$names = $payload | ConvertFrom-Json

$removeCmd = Get-Command Remove-NetAdapter -ErrorAction SilentlyContinue
$disableCmd = Get-Command Disable-NetAdapter -ErrorAction SilentlyContinue
$pnputilCmd = Get-Command pnputil.exe -ErrorAction SilentlyContinue
$getPnpDeviceCmd = Get-Command Get-PnpDevice -ErrorAction SilentlyContinue

function _GetPnpDeviceId([string] $adapterName, [string] $adapterDescription) {
    if (-not $adapterName) {
        return $null
    }
    try {
        $adapter = Get-NetAdapter -Name $adapterName -IncludeHidden -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($adapter -and $adapter.PnPDeviceID) {
            return [string] $adapter.PnPDeviceID
        }
    } catch {
    }
    try {
        $wmi = Get-CimInstance -ClassName Win32_NetworkAdapter -ErrorAction SilentlyContinue |
            Where-Object { $_.NetConnectionID -eq $adapterName } |
            Select-Object -First 1
        if ($wmi -and $wmi.PNPDeviceID) {
            return [string] $wmi.PNPDeviceID
        }
    } catch {
    }
    if ($getPnpDeviceCmd) {
        try {
            $dev = Get-PnpDevice -Class Net -ErrorAction SilentlyContinue |
                Where-Object {
                    ($_.FriendlyName -and ($_.FriendlyName -eq $adapterName -or $_.FriendlyName -like ($adapterName + '*'))) -or
                    ($adapterDescription -and $_.FriendlyName -and ($_.FriendlyName -eq $adapterDescription -or $_.FriendlyName -like ($adapterDescription + '*')))
                } |
                Select-Object -First 1
            if ($dev -and $dev.InstanceId) {
                return [string] $dev.InstanceId
            }
        } catch {
        }
    }
    return $null
}

function _WaitAdapterGone([string] $name, [int] $timeoutSec) {
    $deadline = (Get-Date).AddSeconds($timeoutSec)
    while ((Get-Date) -lt $deadline) {
        $stillThere = Get-NetAdapter -Name $name -IncludeHidden -ErrorAction SilentlyContinue
        if (-not $stillThere) {
            return $true
        }
        Start-Sleep -Milliseconds 250
    }
    return $false
}

foreach ($name in $names) {
    if (-not $name) {
        continue
    }
    $pattern = "$name*"
    $adapters = Get-NetAdapter -Name $pattern -IncludeHidden -ErrorAction SilentlyContinue
    if (-not $adapters) {
        continue
    }
    foreach ($adapter in $adapters) {
        $adapterName = [string] $adapter.Name
        $adapterDescription = [string] $adapter.InterfaceDescription
        if ($removeCmd) {
            Remove-NetAdapter -Name $adapterName -Confirm:$false -ErrorAction SilentlyContinue
            continue
        }
        if ($pnputilCmd) {
            $pnpId = _GetPnpDeviceId $adapterName $adapterDescription
            if ($pnpId) {
                try {
                    & pnputil.exe /remove-device $pnpId /force | Out-Null
                    _WaitAdapterGone -name $adapterName -timeoutSec 10 | Out-Null
                    continue
                } catch {
                }
            }
        }
        if ($disableCmd) {
            Disable-NetAdapter -Name $adapterName -Confirm:$false -ErrorAction SilentlyContinue
        }
    }
}

if ($pnputilCmd) {
    foreach ($name in $names) {
        if (-not $name) {
            continue
        }
        $pattern = "$name*"
        try {
            $wmiAdapters = Get-CimInstance -ClassName Win32_NetworkAdapter -ErrorAction SilentlyContinue |
                Where-Object {
                    ($_.NetConnectionID -and $_.NetConnectionID -like $pattern) -or
                    ($_.Name -and $_.Name -like $pattern)
                }
        } catch {
            $wmiAdapters = @()
        }
        foreach ($wmi in $wmiAdapters) {
            $pnpId = [string] $wmi.PNPDeviceID
            if (-not $pnpId) {
                continue
            }
            try {
                & pnputil.exe /remove-device $pnpId /force | Out-Null
            } catch {
            }
        }
    }
}
exit 0
