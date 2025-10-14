[CmdletBinding()]
param(
  [string]$Interface = 'eth1'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Resolve-PathSafe {
  param([string]$Path)
  (Resolve-Path -LiteralPath $Path).Path
}

function Assert-Command {
  param([string]$Name)
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command '$Name' not found in PATH."
  }
}

$scriptDir = Resolve-PathSafe $PSScriptRoot
$vagrantDirRaw = [System.IO.Path]::Combine($scriptDir, '..', '..', 'infra', 'vagrant-win')
$vagrantDir = Resolve-PathSafe $vagrantDirRaw

Assert-Command -Name 'vagrant'

Push-Location $vagrantDir
try {
  $args = @('ssh','c1','-c',"ip -o -4 addr show dev $Interface")
  $prev = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $output = & vagrant @args 2>&1
    $exit = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $prev
  }

  if ($exit -ne 0) {
    Write-Error "vagrant ssh returned exit code $exit"
    $output | ForEach-Object { Write-Host $_ }
    exit $exit
  }

  $ip = $null
  foreach ($line in $output) {
    if ($line -isnot [string]) { continue }
    $trimmed = $line.Trim()
    if ($trimmed -eq 'Connection to 127.0.0.1 closed.') { continue }
    if ($trimmed -match '\b((?:\d{1,3}\.){3}\d{1,3})/\d+\b') {
      $ip = $Matches[1]
      break
    }
  }

  if ($ip) {
    Write-Host "c1 $Interface IPv4: $ip"
    exit 0
  }
  else {
    Write-Error "IPv4 not found on c1:$Interface"
    $output | ForEach-Object { Write-Host $_ }
    exit 1
  }
}
finally {
  Pop-Location
}
