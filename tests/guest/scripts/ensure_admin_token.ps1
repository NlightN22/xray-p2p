param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$path = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System'
$value = Get-ItemProperty -Path $path -Name 'LocalAccountTokenFilterPolicy' -ErrorAction SilentlyContinue
if (-not $value -or $value.LocalAccountTokenFilterPolicy -ne 1) {
    New-ItemProperty -Path $path -Name 'LocalAccountTokenFilterPolicy' -PropertyType DWord -Value 1 -Force | Out-Null
}

exit 0
