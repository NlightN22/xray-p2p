Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message"
}

function Write-Failure {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Get-ScriptPath {
    if ($PSCommandPath) {
        return $PSCommandPath
    }
    if ($MyInvocation.MyCommand.Path) {
        return $MyInvocation.MyCommand.Path
    }
    throw "Unable to determine script location."
}

function Resolve-PackageRoot {
    param([string]$StartDir)

    $current = $StartDir
    for ($i = 0; $i -lt 5; $i++) {
        if ([string]::IsNullOrWhiteSpace($current)) {
            break
        }
        $manifestCandidate = Join-Path -Path $current -ChildPath "config\deployment.json"
        if (Test-Path -LiteralPath $manifestCandidate) {
            return $current
        }
        $parent = Split-Path -Parent -LiteralPath $current
        if ([string]::IsNullOrWhiteSpace($parent) -or $parent -eq $current) {
            break
        }
        $current = $parent
    }
    throw "xp2p deployment package root not found. Ensure the script runs from the unpacked package."
}

function Read-DeploymentManifest {
    param([string]$ManifestPath)

    if (-not (Test-Path -LiteralPath $ManifestPath)) {
        throw "Deployment manifest not found at $ManifestPath."
    }

    try {
        $content = Get-Content -LiteralPath $ManifestPath -Raw -Encoding UTF8
        $data = $content | ConvertFrom-Json
    } catch {
        throw "Unable to parse deployment manifest at $ManifestPath: $($_.Exception.Message)"
    }

    $version = [string]$data.xp2p_version
    $remoteHost = [string]$data.remote_host
    $installDir = ""
    if ($null -ne $data.install_dir) {
        $installDir = [string]$data.install_dir
    }
    $installDir = $installDir.Trim()

    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "Manifest xp2p_version is missing or empty."
    }
    if ([string]::IsNullOrWhiteSpace($remoteHost)) {
        throw "Manifest remote_host is missing or empty."
    }

    return [PSCustomObject]@{
        Version    = $version.Trim()
        RemoteHost = $remoteHost.Trim()
        InstallDir = $installDir
    }
}

function Get-DefaultInstallDir {
    $programFiles = [Environment]::GetEnvironmentVariable("ProgramFiles")
    if (-not [string]::IsNullOrWhiteSpace($programFiles)) {
        return (Join-Path -Path $programFiles -ChildPath "xp2p")
    }
    return "C:\xp2p"
}

function Ensure-DirectoryWritable {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }

    $probe = Join-Path -Path $Path -ChildPath ("._xp2p_probe_" + [Guid]::NewGuid().ToString("N"))
    try {
        New-Item -Path $probe -ItemType File -Force | Out-Null
    } catch {
        throw "Unable to write to $Path. Verify permissions and rerun as administrator."
    } finally {
        if (Test-Path -LiteralPath $probe) {
            Remove-Item -LiteralPath $probe -Force -ErrorAction SilentlyContinue
        }
    }
}

function New-TemporaryDirectory {
    $base = [IO.Path]::Combine([IO.Path]::GetTempPath(), "xp2p-install-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $base -Force | Out-Null
    return $base
}

function Build-ArtifactUrl {
    param(
        [string]$Version,
        [string]$ArtifactName
    )

    $base = [Environment]::GetEnvironmentVariable("XP2P_RELEASE_BASE_URL")
    if ([string]::IsNullOrWhiteSpace($base)) {
        $base = "https://github.com/NlightN22/xray-p2p/releases/download"
    } else {
        $base = $base.TrimEnd('/')
    }

    return "$base/v$Version/$ArtifactName"
}

function Download-File {
    param(
        [string]$Uri,
        [string]$Destination
    )

    try {
        Invoke-WebRequest -Uri $Uri -OutFile $Destination -UseBasicParsing
    } catch {
        throw "Failed to download $Uri: $($_.Exception.Message)"
    }
}

function Extract-ArchiveTo {
    param(
        [string]$ArchivePath,
        [string]$Destination
    )

    if (-not (Test-Path -LiteralPath $ArchivePath)) {
        throw "Archive $ArchivePath does not exist."
    }

    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $Destination -Force
}

function Resolve-ExtractRoot {
    param([string]$ExtractPath)

    $children = Get-ChildItem -LiteralPath $ExtractPath
    if ($children.Count -eq 1 -and $children[0].PSIsContainer) {
        return $children[0].FullName
    }
    return $ExtractPath
}

function Ensure-Xp2pExecutable {
    param(
        [string]$InstallDir
    )

    $expectedPath = Join-Path -Path $InstallDir -ChildPath "xp2p.exe"
    $matches = Get-ChildItem -LiteralPath $InstallDir -Filter "xp2p.exe" -Recurse

    if ($matches.Count -eq 0) {
        throw "xp2p.exe was not found after extraction."
    }
    if ($matches.Count -gt 1) {
        throw "Multiple xp2p.exe files detected. Clean the install directory and retry."
    }

    $current = $matches[0].FullName
    if (-not (Test-Path -LiteralPath $expectedPath)) {
        if (-not (Test-Path -LiteralPath (Split-Path -Parent -LiteralPath $expectedPath))) {
            New-Item -ItemType Directory -Path (Split-Path -Parent -LiteralPath $expectedPath) -Force | Out-Null
        }
        Move-Item -LiteralPath $current -Destination $expectedPath -Force
    } elseif (-not $current.Equals($expectedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        Move-Item -LiteralPath $current -Destination $expectedPath -Force
    }

    return $expectedPath
}

$tempDir = $null

try {
    Write-Info "xp2p Windows deployment install script started."

    $scriptPath = Get-ScriptPath
    $scriptDir = Split-Path -Parent -LiteralPath $scriptPath
    $packageRoot = Resolve-PackageRoot -StartDir $scriptDir
    $manifestPath = Join-Path -Path $packageRoot -ChildPath "config\deployment.json"

    Write-Info "Reading deployment manifest from $manifestPath."
    $manifest = Read-DeploymentManifest -ManifestPath $manifestPath

    $installDir = if ([string]::IsNullOrWhiteSpace($manifest.InstallDir)) {
        Get-DefaultInstallDir
    } else {
        $manifest.InstallDir
    }
    if (-not [System.IO.Path]::IsPathRooted($installDir)) {
        Write-Info "Manifest provided non-absolute install directory $installDir. Falling back to default."
        $installDir = Get-DefaultInstallDir
    }
    try {
        $installDir = [System.IO.Path]::GetFullPath($installDir)
    }
    catch {
        Write-Info "Manifest provided invalid install directory. Falling back to default."
        $installDir = [System.IO.Path]::GetFullPath((Get-DefaultInstallDir))
    }
    Write-Info "Using install directory $installDir."
    Ensure-DirectoryWritable -Path $installDir

    $artifactName = "xp2p-$($manifest.Version)-windows-amd64.zip"
    $downloadUrl = Build-ArtifactUrl -Version $manifest.Version -ArtifactName $artifactName
    $tempDir = New-TemporaryDirectory
    $archivePath = Join-Path -Path $tempDir -ChildPath $artifactName

    Write-Info "Downloading xp2p $($manifest.Version) from $downloadUrl."
    Download-File -Uri $downloadUrl -Destination $archivePath

    $extractDir = Join-Path -Path $tempDir -ChildPath "extract"
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null

    Write-Info "Extracting xp2p archive."
    Extract-ArchiveTo -ArchivePath $archivePath -Destination $extractDir

    $sourceRoot = Resolve-ExtractRoot -ExtractPath $extractDir
    Write-Info "Deploying xp2p files to $installDir."
    Copy-Item -Path (Join-Path -Path $sourceRoot -ChildPath '*') -Destination $installDir -Recurse -Force

    $xp2pExe = Ensure-Xp2pExecutable -InstallDir $installDir

    $configDirName = "config-server"
    Write-Info "Running xp2p server install."
    $arguments = @(
        "server", "install",
        "--path", $installDir,
        "--config-dir", $configDirName,
        "--host", $manifest.RemoteHost,
        "--deploy-file", $manifestPath,
        "--force"
    )

    & $xp2pExe $arguments
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "xp2p server install failed with exit code $exitCode."
    }

    $binDir = Join-Path -Path $installDir -ChildPath "bin"
    $configDir = Join-Path -Path $installDir -ChildPath $configDirName
    if (-not (Test-Path -LiteralPath $binDir) -or -not (Test-Path -LiteralPath $configDir)) {
        throw "xp2p install verification failed: expected directories missing under $installDir."
    }

    Write-Info "xp2p server install completed successfully."
}
catch {
    Write-Failure $_.Exception.Message
    exit 1
}
finally {
    if ($tempDir -and (Test-Path -LiteralPath $tempDir)) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

exit 0
