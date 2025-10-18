function Run-Prechecks {
    Write-Step 'Pre-setup connectivity checks'
    Invoke-IperfTest -Machine 'r2' -Target '10.0.0.1' -ExpectOpen:$true -Label 'r2 -> r1 (direct WAN)'
    Invoke-IperfTest -Machine 'r3' -Target '10.0.101.1' -ExpectOpen:$false -Label 'r3 -> r1 before tunnel' -AllowAlreadyOpen
}

function Invoke-XSetup {
    param(
        [string]$Machine,
        [string]$ServerAddr,
        [string]$UserName,
        [string]$ServerLan,
        [string]$ClientLan,
        [string]$RepoBaseUrl,
        [string]$SshUser,
        [int]$SshPort,
        [int]$ServerPort,
        [string]$CertFile,
        [string]$KeyFile
    )

    $baseCurl = 'https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh'
    if ($RepoBaseUrl) {
        $trimmed = $RepoBaseUrl.TrimEnd('/', '\')
        $baseCurl = "$trimmed/scripts/xsetup.sh"
    }

    $opts = @()
    if ($SshUser)   { $opts += "-u '" + ($SshUser -replace "'", "'\''") + "'" }
    if ($SshPort)   { $opts += "-p $SshPort" }
    if ($ServerPort){ $opts += "-s $ServerPort" }
    if ($CertFile)  { $opts += "-C '" + ($CertFile -replace "'", "'\''") + "'" }
    if ($KeyFile)   { $opts += "-K '" + ($KeyFile  -replace "'", "'\''") + "'" }

    $envPrefix = ''
    if ($RepoBaseUrl) {
        $envPrefix = "XRAY_REPO_BASE_URL='" + ($RepoBaseUrl -replace "'", "'\''") + "' "
    }

    $optsText = $opts -join ' '
    if (-not [string]::IsNullOrEmpty($optsText)) {
        $optsText += ' '
    }

    $remoteCommand = (
        "curl -fsSL $baseCurl | " +
        $envPrefix +
        "sh -s -- " +
        $optsText +
        "'" + ($ServerAddr -replace "'", "'\''") + "' " +
        "'" + ($UserName  -replace "'", "'\''") + "' " +
        "'" + ($ServerLan -replace "'", "'\''") + "' " +
        "'" + ($ClientLan -replace "'", "'\''") + "'"
    )

    $result = Invoke-VagrantSsh -Machine $Machine -Command $remoteCommand -AllowFail:$true -UseShell

    $resultLines = @()
    if ($result -and $result.PSObject.Properties.Match('Output').Count -gt 0 -and $result.Output) {
        $resultLines = $result.Output
    }
    if ((Get-VerboseLogs) -or $result.ExitCode -ne 0) {
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
            Write-Detail '  [stdout] <empty>'
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
        [string]$Label,
        [switch]$AllowAlreadyOpen
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
    $note = $null
    if ($status -eq 'FAIL' -and $AllowAlreadyOpen -and -not $ExpectOpen -and $actualState -eq 'open') {
        $status = 'PASS'
        $note = 'tunnel already active; accepting open state'
    }

    $displayActual = if ($note) { "$actualState (pre-existing tunnel)" } else { $actualState }
    $outputValue = if ($note) {
        if ([string]::IsNullOrWhiteSpace($outputText)) {
            $note
        }
        else {
            "$note`n$outputText"
        }
    }
    else {
        $outputText
    }

    Add-TestResult ([pscustomobject]@{
        Label    = $Label
        Machine  = $Machine
        Target   = $Target
        Expected = $expectedState
        Actual   = $displayActual
        Status   = $status
        Output   = $outputValue
    })

    $prefix = if ($status -eq 'PASS') { '[PASS]' } else { '[FAIL]' }
    $detailMessage = if ($note) {
        "$prefix $Label (actual=$displayActual; $note)"
    }
    else {
        "$prefix $Label (actual=$actualState)"
    }
    Write-Detail $detailMessage
    if ($status -eq 'FAIL' -and ($outputText -ne '')) {
        Write-Detail "    Output: $outputText"
    }
    elseif ($note -and -not [string]::IsNullOrWhiteSpace($outputText)) {
        Write-Detail "    Output: $outputText"
    }

    return ($status -eq 'PASS')
}

function Invoke-StageR2 {
    param(
        [string]$ServerAddr = '10.0.0.1',
        [string]$UserName   = 'client1',
        [string]$ServerLan  = '10.0.101.0/24',
        [string]$ClientLan  = '10.0.102.0/24',
        [string]$RepoBaseUrl,
        [string]$SshUser,
        [int]$SshPort,
        [int]$ServerPort,
        [string]$CertFile,
        [string]$KeyFile
    )

    Write-Step 'Provisioning tunnel on r2'
    Invoke-XSetup -Machine 'r2' -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan -RepoBaseUrl $RepoBaseUrl -SshUser $SshUser -SshPort $SshPort -ServerPort $ServerPort -CertFile $CertFile -KeyFile $KeyFile | Out-Null
    [void](Invoke-IperfTest -Machine 'r2' -Target '10.0.101.1' -ExpectOpen:$true -Label 'r2 -> r1 after tunnel')

    $c1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
    Write-Detail "Detected c1 eth1 address: $c1Ip"
    [void](Invoke-IperfTest -Machine 'r2' -Target $c1Ip -ExpectOpen:$true -Label 'r2 -> c1 through tunnel')

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
        [string]$C1Ip,
        [string]$RepoBaseUrl,
        [string]$SshUser,
        [int]$SshPort,
        [int]$ServerPort,
        [string]$CertFile,
        [string]$KeyFile
    )

    Write-Step 'Provisioning tunnel on r3'
    if (-not $C1Ip) {
        $C1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
        Write-Detail "Detected c1 eth1 address for r3 stage: $C1Ip"
    }

    Invoke-XSetup -Machine 'r3' -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan -RepoBaseUrl $RepoBaseUrl -SshUser $SshUser -SshPort $SshPort -ServerPort $ServerPort -CertFile $CertFile -KeyFile $KeyFile | Out-Null
    [void](Invoke-IperfTest -Machine 'r3' -Target '10.0.101.1' -ExpectOpen:$true -Label 'r3 -> r1 after tunnel')
    [void](Invoke-IperfTest -Machine 'r3' -Target $C1Ip -ExpectOpen:$true -Label 'r3 -> c1 through tunnel')

    $c2Ip = Get-InterfaceIpv4 -Machine 'c2' -Interface 'eth1'
    $c3Ip = Get-InterfaceIpv4 -Machine 'c3' -Interface 'eth1'
    Write-Detail "Detected c2 eth1 address: $c2Ip"
    Write-Detail "Detected c3 eth1 address: $c3Ip"

    [void](Invoke-IperfTest -Machine 'c3' -Target $C1Ip -ExpectOpen:$true -Label 'c3 -> c1 client reachability')

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

    Write-Step 'Reverse tunnel validation'
    [void](Invoke-IperfTest -Machine 'r1' -Target $C2Ip -ExpectOpen:$true -Label 'r1 -> c2 reverse tunnel')
    [void](Invoke-IperfTest -Machine 'r1' -Target $C3Ip -ExpectOpen:$true -Label 'r1 -> c3 reverse tunnel')
    [void](Invoke-IperfTest -Machine 'c1' -Target $C2Ip -ExpectOpen:$true -Label 'c1 -> c2 reverse tunnel')
    [void](Invoke-IperfTest -Machine 'c1' -Target $C3Ip -ExpectOpen:$true -Label 'c1 -> c3 reverse tunnel')
}
