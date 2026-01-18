param(
    [int] $Samples = 5
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($Samples -lt 1) {
    $Samples = 1
}

$results = @()
for ($i = 0; $i -lt $Samples; $i += 1) {
    $ms = (Measure-Command {
        powershell -NoProfile -NonInteractive -NoLogo -Command "1" | Out-Null
    }).TotalMilliseconds
    $results += [double]$ms
}

$avg = ($results | Measure-Object -Average).Average
$min = ($results | Measure-Object -Minimum).Minimum
$max = ($results | Measure-Object -Maximum).Maximum

$sorted = $results | Sort-Object
$p95Index = [Math]::Ceiling(0.95 * $sorted.Count) - 1
if ($p95Index -lt 0) { $p95Index = 0 }
$p95 = $sorted[$p95Index]

Write-Output ("samples_ms={0}" -f ($results -join ","))
Write-Output ("avg_ms={0:N2}" -f $avg)
Write-Output ("min_ms={0:N2}" -f $min)
Write-Output ("max_ms={0:N2}" -f $max)
Write-Output ("p95_ms={0:N2}" -f $p95)
