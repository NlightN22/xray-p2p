$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

try {
    $policyPath = "HKLM:\SOFTWARE\Policies\Microsoft\Windows\Windows Feeds"
    New-Item -Path $policyPath -Force | Out-Null
    Set-ItemProperty -Path $policyPath -Name "EnableFeeds" -Type DWord -Value 0

    Stop-Process -Name explorer -Force -ErrorAction SilentlyContinue
} catch {
    Write-Host ("[disable-windows-feeds] warning: {0}" -f ($_.Exception.Message))
}
