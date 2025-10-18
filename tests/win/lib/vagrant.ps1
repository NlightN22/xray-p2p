function Invoke-Vagrant {
    param(
        [string[]]$Arguments,
        [switch]$CaptureOutput,
        [switch]$AllowFail
    )

    $context = Get-TestContext
    Push-Location $context.VagrantDir
    Write-Detail "Running: vagrant $($Arguments -join ' ')"
    $previousErrorPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        if ($CaptureOutput) {
            $rawOutput = & $context.VagrantExe @Arguments 2>&1
            $exitCode = $LASTEXITCODE
            $output = @()
            foreach ($item in $rawOutput) {
                if ($null -eq $item) { continue }
                $lineText = if ($item -is [string]) {
                    $item
                }
                elseif ($item -is [System.Management.Automation.ErrorRecord]) {
                    $item.ToString()
                }
                else {
                    ($item | Out-String).TrimEnd()
                }
                if ($lineText -ne $null) {
                    $output += $lineText
                }
            }
            if ((Get-VerboseLogs) -or $exitCode -ne 0) {
                foreach ($line in $output) {
                    Write-Host $line
                }
            }
        }
        else {
            & $context.VagrantExe @Arguments
            $exitCode = $LASTEXITCODE
            $output = @()
        }
    }
    finally {
        $ErrorActionPreference = $previousErrorPreference
        Pop-Location
    }

    if (-not $AllowFail -and $exitCode -ne 0) {
        throw "Command 'vagrant $($Arguments -join ' ')' failed with exit code $exitCode."
    }

    [pscustomobject]@{
        ExitCode = $exitCode
        Output   = if ($output -is [Array]) { $output } else { @($output) }
    }
}

function Invoke-VagrantSsh {
    param(
        [string]$Machine,
        [string]$Command,
        [switch]$AllowFail,
        [switch]$CaptureOutput = $true,
        [switch]$UseShell
    )

    $args = @('ssh', $Machine, '-c', $Command)
    Invoke-Vagrant -Arguments $args -CaptureOutput:$CaptureOutput -AllowFail:$AllowFail
}

function Ensure-VagrantBox {
    $boxList = Invoke-Vagrant -Arguments @('box', 'list') -CaptureOutput
    $boxPresent = $false
    foreach ($line in $boxList.Output) {
        if ($line -match '^\s*openwrt-gw\s') {
            $boxPresent = $true
            break
        }
    }
    if ($boxPresent) {
        Write-Detail "Vagrant box 'openwrt-gw' already installed."
        return
    }

    $context = Get-TestContext
    $boxPath = Resolve-PathSafe (Join-Path $context.VagrantDir 'openwrt-gw.box')
    Write-Detail "Adding Vagrant box from $boxPath."
    Invoke-Vagrant -Arguments @('box', 'add', 'openwrt-gw', $boxPath)
}

function Wait-ForSsh {
    param(
        [string]$Machine,
        [int]$Retries = 5
    )

    for ($attempt = 1; $attempt -le $Retries; $attempt++) {
        $result = Invoke-VagrantSsh -Machine $Machine -Command 'true' -AllowFail:$true
        if ($result.ExitCode -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "Unable to establish SSH session to $Machine after $Retries attempts."
}

function Start-VagrantEnvironment {
    param(
        [int]$MaxAttempts = 2,
        [bool]$DestroyBetweenAttempts = $true
    )

    Write-Step 'Ensuring Vagrant base box'
    Ensure-VagrantBox

    Write-Step 'Booting Vagrant environment'
    $upAttempt = 0
    $upResult = $null
    while ($upAttempt -lt $MaxAttempts) {
        $upAttempt++
        if ($upAttempt -gt 1) {
            Write-Detail "Retry attempt $upAttempt for 'vagrant up'."
        }
        $upResult = Invoke-Vagrant -Arguments @('up', '--no-parallel') -CaptureOutput -AllowFail:$true
        if ($upResult.ExitCode -eq 0) {
            return $upResult
        }

        if ($DestroyBetweenAttempts -and $upAttempt -lt $MaxAttempts) {
            Write-Detail "'vagrant up' failed (exit $($upResult.ExitCode)); attempting environment reset via 'vagrant destroy -f'."
            Invoke-Vagrant -Arguments @('destroy', '-f') -CaptureOutput -AllowFail:$true | Out-Null
            Start-Sleep -Seconds 5
            continue
        }

        break
    }

    throw "Command 'vagrant up' failed with exit code $($upResult.ExitCode) after $MaxAttempts attempts."
}

function Test-SshReadiness {
    param(
        [string[]]$Machines = @('r1','r2','r3','c1','c2','c3')
    )

    Write-Step 'Checking SSH access to all nodes'
    foreach ($machine in $Machines) {
        Write-Detail "Waiting for $machine..."
        Wait-ForSsh -Machine $machine
    }
}
