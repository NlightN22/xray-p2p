param(
    [Parameter(Mandatory = $true)]
    [string]$ServicesBase64
)

$ErrorActionPreference = 'Stop'
$payload = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($ServicesBase64))
$services = $payload | ConvertFrom-Json

foreach ($name in $services) {
    if (-not $name) {
        continue
    }
    Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    & sc.exe delete $name | Out-Null
}
exit 0
