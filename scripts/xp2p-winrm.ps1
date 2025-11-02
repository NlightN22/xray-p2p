[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [string]$VM = "win10-server",
    [Parameter(Mandatory = $true)]
    [string]$CMD
)

$ErrorActionPreference = "Stop"

function ConvertTo-SingleQuoted {
    param([string]$Value)
    return "'" + ($Value -replace "'", "''") + "'"
}

function Format-ArgumentForDisplay {
    param([string]$Argument)
    if ($Argument -match "[\s'`"]") {
        return '"' + ($Argument -replace '"', '\"') + '"'
    }
    return $Argument
}

function Get-CommandArguments {
    param([string]$CommandLine)

    if ([string]::IsNullOrWhiteSpace($CommandLine)) {
        throw "CMD parameter cannot be empty."
    }

    $errors = @()
    $tokens = @()
    $parsed = [System.Management.Automation.Language.Parser]::ParseInput("dummy $CommandLine", [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        $messages = $errors | ForEach-Object { $_.Message }
        throw "Unable to parse CMD: $($messages -join '; ')"
    }

    $commandAst = $parsed.FindAll({ param($node) $node -is [System.Management.Automation.Language.CommandAst] }, $true) | Select-Object -First 1
    if (-not $commandAst) {
        throw "Unable to extract arguments from CMD."
    }

    $elements = $commandAst.CommandElements | Select-Object -Skip 1
    if (-not $elements) {
        throw "CMD must include at least one argument."
    }

    return @(
        $elements | ForEach-Object {
            $value = $_.SafeGetValue()
            if ($null -eq $value) {
                $_.Extent.Text
            } else {
                $value.ToString()
            }
        }
    )
}

try {
    $repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
    Write-Host "==> Repository root: $repoRoot"

    $exeRelative = "build/windows-amd64/xp2p.exe"
    $exeHostPath = Join-Path $repoRoot $exeRelative
    if (-not (Test-Path $exeHostPath)) {
        throw "xp2p.exe was not produced at $exeHostPath."
    }

    Write-Host "==> Located host binary at $exeHostPath"

    $vagrantRoot = Join-Path $repoRoot "infra/vagrant-win"
    if (-not (Test-Path $vagrantRoot)) {
        throw "Vagrant root directory not found at $vagrantRoot."
    }

    $machineDir = $null
    foreach ($dir in Get-ChildItem -Directory -Path $vagrantRoot) {
        $vagrantFile = Join-Path $dir.FullName "Vagrantfile"
        if (-not (Test-Path $vagrantFile)) {
            continue
        }

        $pattern = "config\.vm\.define\s+`"" + [Regex]::Escape($VM) + "`""
        if (Select-String -Path $vagrantFile -Pattern $pattern -Quiet) {
            $machineDir = $dir.FullName
            break
        }
    }

    if (-not $machineDir) {
        throw "Unable to find Vagrant environment containing machine '$VM'."
    }

    Write-Host "==> Using Vagrant environment at $machineDir"

    $arguments = Get-CommandArguments -CommandLine $CMD
    $displayParts = @("xp2p.exe")
    $displayParts += ($arguments | ForEach-Object { Format-ArgumentForDisplay -Argument $_ })
    $displayCommand = $displayParts -join " "
    Write-Host "==> Prepared remote command: $displayCommand"

    $remoteExePath = "C:\vagrant\build\windows-amd64\xp2p.exe"
    $remoteArgsLiteral = ""
    if ($arguments.Count -gt 0) {
        $remoteArgsLiteral = ($arguments | ForEach-Object { ConvertTo-SingleQuoted $_ }) -join ", "
    }

    $remoteLines = @(
        '$ErrorActionPreference = ''Stop'''
        '$exe = ' + (ConvertTo-SingleQuoted $remoteExePath)
        'if (-not (Test-Path $exe)) { throw ''xp2p executable not found at '' + $exe }'
        '$info = Get-Item $exe'
        'Write-Host ("==> Using " + $info.FullName + " (LastWriteTime: " + $info.LastWriteTime.ToString("u") + ")")'
        '$argsList = @(' + $remoteArgsLiteral + ')'
        'Write-Host ' + (ConvertTo-SingleQuoted "==> Executing: $displayCommand")
        '& $exe @argsList'
        'exit $LASTEXITCODE'
    )

    $remoteScript = [string]::Join([Environment]::NewLine, $remoteLines)
    $encodedCommand = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($remoteScript))
    $remoteCommand = "powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -EncodedCommand $encodedCommand"

    Push-Location $machineDir
    try {
        Write-Host "==> Starting VM '$VM' if required"
        & vagrant up $VM --no-provision
        if ($LASTEXITCODE -ne 0) {
            throw "vagrant up failed with exit code $LASTEXITCODE."
        }

        Write-Host "==> Running xp2p on '$VM'"
        & vagrant winrm $VM --command $remoteCommand
        $remoteExitCode = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    if ($remoteExitCode -ne 0) {
        throw "xp2p execution failed with exit code $remoteExitCode."
    }

    Write-Host "==> Remote execution completed successfully"
    exit 0
} catch {
    Write-Error $_
    exit 1
}
