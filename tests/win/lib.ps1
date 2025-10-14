Set-StrictMode -Version Latest

if (-not (Get-Variable -Name LibraryRoot -Scope Script -ErrorAction SilentlyContinue)) {
    $script:LibraryRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
}

$script:VerboseLogs = $false
if (-not (Get-Variable -Name TestResults -Scope Script -ErrorAction SilentlyContinue)) {
    $script:TestResults = @()
}

function Set-VerboseLogs {
    param([bool]$Enabled)
    $script:VerboseLogs = $Enabled
}

function Reset-TestResults {
    $script:TestResults = @()
}

function Get-TestResults {
    return @($script:TestResults)
}

function Publish-TestResults {
    param()
    $results = Get-TestResults
    Write-Step "Test Summary"
    if (-not $results -or $results.Count -eq 0) {
        Write-Detail "No tests recorded."
        return $results
    }

    foreach ($res in $results) {
        $prefix = if ($res.Status -eq 'PASS') { '[PASS]' } else { '[FAIL]' }
        $message = "{0} {1} [{2} -> {3}] expected={4} actual={5}" -f $prefix, $res.Label, $res.Machine, $res.Target, $res.Expected, $res.Actual
        Write-Detail $message
        if ($res.Status -eq 'FAIL' -and $script:VerboseLogs -and $res.Output) {
            Write-Detail "    Output: $($res.Output)"
        }
    }

    return $results
}

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

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Write-Detail {
    param([string]$Message)
    Write-Host "    $Message"
}

$script:TestContext = $null

function Get-TestContext {
    if ($null -eq $script:TestContext) {
        $scriptDir   = Resolve-PathSafe $script:LibraryRoot
        $testsDir    = (Get-Item $scriptDir).Parent
        if (-not $testsDir) {
            throw "Unable to locate tests directory from $scriptDir."
        }
        $repoRoot    = $testsDir.Parent
        if (-not $repoRoot) {
            throw "Unable to locate repository root from $testsDir."
        }
        $repoRootPath = $repoRoot.FullName
        $vagrantDir   = Resolve-PathSafe (Join-Path $repoRootPath 'infra\vagrant-win')

        Assert-Command -Name 'vagrant'
        $vagrantExe = (Get-Command 'vagrant').Source

        $env:VAGRANT_DISABLE_STRICT_HOST_KEY_CHECKING = '1'

        $script:TestContext = [pscustomobject]@{
            ScriptDir = $scriptDir
            TestsDir  = $testsDir.FullName
            RepoRoot  = $repoRootPath
            VagrantDir = $vagrantDir
            VagrantExe = $vagrantExe
        }
    }

    return $script:TestContext
}

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
            if ($script:VerboseLogs -or $exitCode -ne 0) {
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

function Invoke-XSetup {
    param(
        [string]$Machine,
        [string]$ServerAddr,
        [string]$UserName,
        [string]$ServerLan,
        [string]$ClientLan
    )

    $remoteCommand = [string]::Format(
        "if ! command -v ssh >/dev/null 2>&1 || ssh -V 2>&1 | grep -iq dropbear; then opkg update >/dev/null 2>&1; opkg install openssh-client >/dev/null 2>&1 || opkg install openssh-client; fi; curl -fsSL https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh | sh -s -- '{0}' '{1}' '{2}' '{3}'",
        $ServerAddr,
        $UserName,
        $ServerLan,
        $ClientLan
    )

    $result = Invoke-VagrantSsh -Machine $Machine -Command $remoteCommand -AllowFail:$true -UseShell

    $resultLines = @()
    if ($result -and $result.PSObject.Properties.Match('Output').Count -gt 0 -and $result.Output) {
        $resultLines = $result.Output
    }
    if ($script:VerboseLogs -or $result.ExitCode -ne 0) {
        foreach ($line in $resultLines) {
            Write-Host "    [$Machine] $line"
        }
    }
    if ($result.ExitCode -ne 0) {
        throw "xsetup execution on $Machine failed with exit code $($result.ExitCode)."
    }

    $trojanUrl = ''
    foreach ($line in $resultLines) {
        if ($line -match '__TROJAN_LINK__=(.+)$') {
            $trojanUrl = $Matches[1]
        }
    }

    [pscustomobject]@{
        TrojanUrl = $trojanUrl
    }
}

function Get-InterfaceIpv4 {
    param(
        [string]$Machine,
        [string]$Interface,
        [int]$Retries = 6
    )

    $command = "ip -o -4 addr show dev $Interface"
    for ($attempt = 1; $attempt -le $Retries; $attempt++) {
        $result = Invoke-VagrantSsh -Machine $Machine -Command $command -AllowFail:$true
        $resultType = if ($null -eq $result) { '<null>' } else { $result.GetType().FullName }
        $outputLines = @()
        if ($result -and $result.PSObject.Properties.Match('Output').Count -gt 0 -and $result.Output) {
            $outputLines = $result.Output
        }
        $exitCode = if ($result -and $result.PSObject.Properties.Match('ExitCode').Count -gt 0) { $result.ExitCode } else { '<unknown>' }
        Write-Detail "Attempt $attempt/$Retries -> ${Machine}:${Interface} exit $exitCode (type $resultType)"
        if ($outputLines.Count -gt 0) {
            foreach ($line in $outputLines) {
                Write-Detail "  [stdout] $line"
            }
        }
        else {
            Write-Detail "  [stdout] <empty>"
        }

        $candidateIps = @()
        foreach ($line in $outputLines) {
            if ($line -isnot [string]) { continue }
            $trimmed = $line.Trim()
            if ($trimmed -eq 'Connection to 127.0.0.1 closed.') { continue }
            if ($trimmed -match '\b(?<ip>(?:\d{1,3}\.){3}\d{1,3})/\d+\b') {
                $candidateIps += $Matches.ip
            }
        }
        if ($candidateIps.Count -gt 0) {
            return $candidateIps[0]
        }

        if ($attempt -lt $Retries) {
            Write-Detail "No IPv4 detected on ${Machine}:${Interface} (attempt $attempt/$Retries); retrying DHCP."
            Invoke-VagrantSsh -Machine $Machine -Command "ip link set $Interface up && udhcpc -i $Interface -t 3 -T 3 -n" -AllowFail:$true -CaptureOutput:$false -UseShell | Out-Null
            Start-Sleep -Seconds 2
        }
    }
    throw "Failed to detect IPv4 address on ${Machine}:${Interface}."
}

function Invoke-IperfTest {
    param(
        [string]$Machine,
        [string]$Target,
        [bool]$ExpectOpen,
        [string]$Label
    )

    $expectedState = if ($ExpectOpen) { 'open' } else { 'closed' }
    Write-Detail "$Label => expecting $expectedState"
    $iperfCmd = "iperf3 -c $Target -t 1 -P 1 >/dev/null 2>&1 && echo open || { echo closed; false; }"
    $result = Invoke-VagrantSsh -Machine $Machine -Command $iperfCmd -AllowFail:$true
    $outputLines = @()
    if ($result -and $result.PSObject.Properties.Match('Output').Count -gt 0 -and $result.Output) {
        $outputLines = $result.Output
    }
    $outputText = ($outputLines | ForEach-Object {
        if ($_ -is [string]) {
            $_.TrimEnd()
        }
        elseif ($_ -is [System.Management.Automation.ErrorRecord]) {
            $_.ToString().TrimEnd()
        }
    }) -join "`n"

    $actualState = if ($result.ExitCode -eq 0) { 'open' } else { 'closed' }
    $status = if ($actualState -eq $expectedState) { 'PASS' } else { 'FAIL' }

    $script:TestResults += [pscustomobject]@{
        Label    = $Label
        Machine  = $Machine
        Target   = $Target
        Expected = $expectedState
        Actual   = $actualState
        Status   = $status
        Output   = $outputText
    }

    $prefix = if ($status -eq 'PASS') { '[PASS]' } else { '[FAIL]' }
    Write-Detail "$prefix $Label (actual=$actualState)"
    if ($status -eq 'FAIL' -and ($outputText -ne '')) {
        Write-Detail "    Output: $outputText"
    }

    return ($status -eq 'PASS')
}

function Start-VagrantEnvironment {
    param(
        [int]$MaxAttempts = 2,
        [bool]$DestroyBetweenAttempts = $true
    )

    Write-Step "Ensuring Vagrant base box"
    Ensure-VagrantBox

    Write-Step "Booting Vagrant environment"
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

    Write-Step "Checking SSH access to all nodes"
    foreach ($machine in $Machines) {
        Write-Detail "Waiting for $machine..."
        Wait-ForSsh -Machine $machine
    }
}

function Run-Prechecks {
    Write-Step "Pre-setup connectivity checks"
    Invoke-IperfTest -Machine 'r2' -Target '10.0.0.1' -ExpectOpen:$false -Label 'r2 -> r1 (direct WAN)'
    Invoke-IperfTest -Machine 'r3' -Target '10.0.101.1' -ExpectOpen:$false -Label 'r3 -> r1 before tunnel'
}

function Invoke-StageR2 {
    param(
        [string]$ServerAddr = '10.0.0.1',
        [string]$UserName   = 'client1',
        [string]$ServerLan  = '10.0.101.0/24',
        [string]$ClientLan  = '10.0.102.0/24'
    )

    Write-Step "Provisioning tunnel on r2"
    Invoke-XSetup -Machine 'r2' -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan | Out-Null
    Invoke-IperfTest -Machine 'r2' -Target '10.0.101.1' -ExpectOpen:$true -Label 'r2 -> r1 after tunnel'

    $c1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
    Write-Detail "Detected c1 eth1 address: $c1Ip"
    Invoke-IperfTest -Machine 'r2' -Target $c1Ip -ExpectOpen:$true -Label 'r2 -> c1 through tunnel'

    [pscustomobject]@{
        C1Ip = $c1Ip
    }
}

function Invoke-StageR3 {
    param(
        [string]$ServerAddr = '10.0.0.1',
        [string]$UserName   = 'client3',
        [string]$ServerLan  = '10.0.101.0/24',
        [string]$ClientLan  = '10.0.103.0/24',
        [string]$C1Ip
    )

    Write-Step "Provisioning tunnel on r3"
    if (-not $C1Ip) {
        $C1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
        Write-Detail "Detected c1 eth1 address for r3 stage: $C1Ip"
    }

    Invoke-XSetup -Machine 'r3' -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan | Out-Null
    Invoke-IperfTest -Machine 'r3' -Target '10.0.101.1' -ExpectOpen:$true -Label 'r3 -> r1 after tunnel'
    Invoke-IperfTest -Machine 'r3' -Target $C1Ip -ExpectOpen:$true -Label 'r3 -> c1 through tunnel'

    $c2Ip = Get-InterfaceIpv4 -Machine 'c2' -Interface 'eth1'
    $c3Ip = Get-InterfaceIpv4 -Machine 'c3' -Interface 'eth1'
    Write-Detail "Detected c2 eth1 address: $c2Ip"
    Write-Detail "Detected c3 eth1 address: $c3Ip"

    Invoke-IperfTest -Machine 'c3' -Target $C1Ip -ExpectOpen:$true -Label 'c3 -> c1 client reachability'

    [pscustomobject]@{
        C1Ip = $C1Ip
        C2Ip = $c2Ip
        C3Ip = $c3Ip
    }
}

function Test-ReverseTunnels {
    param(
        [Parameter(Mandatory=$true)][string]$C1Ip,
        [Parameter(Mandatory=$true)][string]$C2Ip,
        [Parameter(Mandatory=$true)][string]$C3Ip
    )

    Write-Step "Reverse tunnel validation"
    Invoke-IperfTest -Machine 'r1' -Target $C2Ip -ExpectOpen:$true -Label 'r1 -> c2 reverse tunnel'
    Invoke-IperfTest -Machine 'r1' -Target $C3Ip -ExpectOpen:$true -Label 'r1 -> c3 reverse tunnel'
    Invoke-IperfTest -Machine 'c1' -Target $C2Ip -ExpectOpen:$true -Label 'c1 -> c2 reverse tunnel'
    Invoke-IperfTest -Machine 'c1' -Target $C3Ip -ExpectOpen:$true -Label 'c1 -> c3 reverse tunnel'
}



