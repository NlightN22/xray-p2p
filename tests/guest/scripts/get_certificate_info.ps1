param(
    [Parameter(Mandatory = $true)]
    [string] $Path
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

try {
    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Error "Certificate not found at path '$Path'"
        exit 3
    }

    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($bytes)

    $subjectCN = $cert.GetNameInfo(
        [System.Security.Cryptography.X509Certificates.X509NameType]::SimpleName,
        $false
    )
    $notAfter = $cert.NotAfter.ToUniversalTime().ToString("o")

    $sanEntries = @()
    $sanExt = $cert.Extensions | Where-Object { $_.Oid.Value -eq "2.5.29.17" }
    if ($sanExt) {
        $formatted = $sanExt.Format($false)
        $normalized = $formatted -replace '\s*\r?\n\s*', ', '
        foreach ($entry in ($normalized -split '\s*,\s*')) {
            if ([string]::IsNullOrWhiteSpace($entry)) {
                continue
            }
            if ($entry -match '^(DNS Name|IP Address)=(.+)$') {
                $sanEntries += [pscustomobject]@{
                    Type  = $matches[1]
                    Value = $matches[2]
                }
            }
        }
    }

    $result = [pscustomobject]@{
        SubjectCN      = $subjectCN
        NotAfter       = $notAfter
        SubjectAltName = $sanEntries
    }
    $result | ConvertTo-Json -Compress
    exit 0
}
catch {
    Write-Error $_.Exception.Message
    exit 1
}
