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

function Get-TestContext {
    $context = Get-StateValue -Name 'TestContext'
    if ($null -ne $context) {
        return $context
    }

    $scriptDir = Resolve-PathSafe (Get-LibraryRoot)
    $testsDir = (Get-Item $scriptDir).Parent
    if (-not $testsDir) {
        throw "Unable to locate tests directory from $scriptDir."
    }
    $repoRoot = $testsDir.Parent
    if (-not $repoRoot) {
        throw "Unable to locate repository root from $testsDir."
    }

    $repoRootPath = $repoRoot.FullName
    $vagrantDir = Resolve-PathSafe (Join-Path $repoRootPath 'infra\vagrant-win')

    Assert-Command -Name 'vagrant'
    $vagrantExe = (Get-Command 'vagrant').Source

    $env:VAGRANT_DISABLE_STRICT_HOST_KEY_CHECKING = '1'

    $context = [pscustomobject]@{
        ScriptDir  = $scriptDir
        TestsDir   = $testsDir.FullName
        RepoRoot   = $repoRootPath
        VagrantDir = $vagrantDir
        VagrantExe = $vagrantExe
    }

    Set-StateValue -Name 'TestContext' -Value $context
    return $context
}
