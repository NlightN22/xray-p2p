function Get-VerboseLogs {
    $value = Get-StateValue -Name 'VerboseLogs'
    if ($null -eq $value) {
        return $false
    }
    return [bool]$value
}

function Set-VerboseLogs {
    param([bool]$Enabled)
    Set-StateValue -Name 'VerboseLogs' -Value $Enabled
}

function Reset-TestResults {
    $script:XrayWinState['TestResults'] = [System.Collections.ArrayList]::new()
}

function Get-TestResults {
    $list = $script:XrayWinState['TestResults']
    if ($null -eq $list -or $list -isnot [System.Collections.ArrayList]) {
        $list = [System.Collections.ArrayList]::new()
        $script:XrayWinState['TestResults'] = $list
    }
    if ($list.Count -eq 0) {
        return
    }
    return $list.ToArray()
}

function Add-TestResult {
    param([pscustomobject]$Result)
    $list = $script:XrayWinState['TestResults']
    if ($null -eq $list -or $list -isnot [System.Collections.ArrayList]) {
        $list = [System.Collections.ArrayList]::new()
        $script:XrayWinState['TestResults'] = $list
    }
    [void]$list.Add($Result)
}

function Publish-TestResults {
    $items = @(Get-TestResults)
    Write-Step 'Test Summary'
    if ($items.Count -eq 0) {
        Write-Detail 'No tests recorded.'
        return @()
    }

    foreach ($res in $items) {
        $prefix = if ($res.Status -eq 'PASS') { '[PASS]' } else { '[FAIL]' }
        $message = "{0} {1} [{2} -> {3}] expected={4} actual={5}" -f $prefix, $res.Label, $res.Machine, $res.Target, $res.Expected, $res.Actual
        Write-Detail $message
        if ($res.Status -eq 'FAIL' -and (Get-VerboseLogs) -and $res.Output) {
            Write-Detail "    Output: $($res.Output)"
        }
    }

    return $items
}

function Write-Step {
    param([string]$Message)
    Write-Host ''
    Write-Host "==> $Message"
}

function Write-Detail {
    param([string]$Message)
    Write-Host "    $Message"
}
