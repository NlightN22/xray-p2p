param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $Xp2pUiPath = "",
    [string] $ServiceNamesBase64 = "",
    [int] $UiWaitSeconds = 5,
    [int] $ServiceWaitSeconds = 20
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Ensure-Marker([string] $path, [string] $payload) {
    if (-not $path) {
        return
    }
    $dir = Split-Path -Parent $path
    if ($dir) {
        [System.IO.Directory]::CreateDirectory($dir) | Out-Null
    }
    Set-Content -Path $path -Value $payload -Encoding ASCII
}

function Wait-ServiceState([string] $name, [string] $expected, [int] $timeoutSeconds) {
    $deadline = [DateTime]::UtcNow.AddSeconds($timeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) {
            throw "Service $name not found"
        }
        if ($svc.Status.ToString().ToLowerInvariant() -eq $expected.ToLowerInvariant()) {
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "Service $name did not reach state $expected within ${timeoutSeconds}s"
}

function Invoke-ScAsUser([System.Management.Automation.PSCredential] $cred, [string[]] $args) {
    $proc = Start-Process -FilePath "sc.exe" -ArgumentList $args -Credential $cred -PassThru -Wait
    if ($proc.ExitCode -ne 0) {
        throw "sc.exe failed with exit code $($proc.ExitCode) for args: $($args -join ' ')"
    }
}

$exitCode = 0
$userName = $null
$password = $null
$uiProc = $null

try {
    if ($Xp2pUiPath) {
        if (-not (Test-Path $Xp2pUiPath)) {
            Write-Output "xp2p-ui not found at $Xp2pUiPath"
            $exitCode = 3
            return
        }
        $uiProc = Start-Process -FilePath $Xp2pUiPath -PassThru
        if ($UiWaitSeconds -gt 0) {
            Start-Sleep -Seconds $UiWaitSeconds
        }
        $uiRunning = Get-Process -Id $uiProc.Id -ErrorAction SilentlyContinue
        if (-not $uiRunning) {
            Write-Output "xp2p-ui exited before service toggle"
            $exitCode = 4
            return
        }
        Stop-Process -Id $uiProc.Id -Force -ErrorAction SilentlyContinue
    }

    $serviceNames = @()
    if ($ServiceNamesBase64) {
        $decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($ServiceNamesBase64))
        $serviceNames = $decoded | ConvertFrom-Json
    }
    if (-not $serviceNames -or $serviceNames.Count -eq 0) {
        $serviceNames = @("xp2p-client", "xp2p-server")
    }

    foreach ($name in $serviceNames) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) {
            Write-Output "Service $name not found"
            $exitCode = 5
            return
        }
    }

    $userName = "xp2p-ui-test-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
    $password = "Xp2pUi!" + (Get-Random -Minimum 100000 -Maximum 999999).ToString() + "Aa"
    $userPath = "$env:COMPUTERNAME\$userName"
    & net user $userName $password /add | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create local user $userName (exit $LASTEXITCODE)"
    }

    $secure = ConvertTo-SecureString $password -AsPlainText -Force
    $cred = New-Object System.Management.Automation.PSCredential($userPath, $secure)

    foreach ($name in $serviceNames) {
        $initial = (Get-Service -Name $name).Status.ToString()
        if ($initial -eq "Running") {
            Invoke-ScAsUser $cred @("stop", $name)
            Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
            Invoke-ScAsUser $cred @("start", $name)
            Wait-ServiceState $name "Running" $ServiceWaitSeconds
        } else {
            Invoke-ScAsUser $cred @("start", $name)
            Wait-ServiceState $name "Running" $ServiceWaitSeconds
            Invoke-ScAsUser $cred @("stop", $name)
            Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
        }
    }
} catch {
    Write-Output "toggle_service_via_ui failed: $($_.Exception.Message)"
    $exitCode = 10
} finally {
    if ($uiProc -and -not $uiProc.HasExited) {
        Stop-Process -Id $uiProc.Id -Force -ErrorAction SilentlyContinue
    }
    if ($userName) {
        & net user $userName /delete | Out-Null
    }
    $payload = if ($exitCode -eq 0) { "OK" } else { "FAIL:$exitCode" }
    Ensure-Marker $MarkerPath $payload
}

exit $exitCode
