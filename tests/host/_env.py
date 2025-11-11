import base64
import shutil
import subprocess
from functools import lru_cache
from pathlib import Path
from typing import Iterable

import pytest
import testinfra
from testinfra.backend.base import CommandResult
from testinfra.host import Host

REPO_ROOT = Path(__file__).resolve().parents[2]
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant-win" / "windows10"
DEFAULT_SERVER = "win10-server"
DEFAULT_CLIENT = "win10-client"
PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
XP2P_EXE = PROGRAM_FILES_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60
GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")
MSI_MARKER = "__MSI_PATH__="

_MSI_CACHE_PATH: str | None = None


def require_vagrant_environment() -> None:
    if shutil.which("vagrant") is None:
        pytest.skip("Vagrant executable not found on host; guest tests are unavailable.")
    if not VAGRANT_DIR.exists():
        pytest.skip(
            f"Expected Vagrant environment at '{VAGRANT_DIR}' is missing; "
            "run `make vagrant-win10` before invoking host tests."
        )


def ensure_machine_running(machine: str) -> None:
    try:
        state = machine_state(machine)
    except subprocess.CalledProcessError as exc:
        pytest.skip(
            f"Unable to determine state for guest '{machine}' "
            f"(vagrant status exited with code {exc.returncode}). "
            "Run `make vagrant-win10` and retry."
        )
    if state != "running":
        pytest.skip(
            f"Guest '{machine}' is not running (state={state!r}). "
            "Run `make vagrant-win10` and retry."
        )


@lru_cache(maxsize=8)
def machine_state(machine: str) -> str | None:
    output = subprocess.check_output(
        ["vagrant", "status", machine, "--machine-readable"],
        cwd=VAGRANT_DIR,
        text=True,
    )
    for line in output.splitlines():
        parts = line.split(",")
        if len(parts) >= 4 and parts[2] == "state":
            return parts[3]
    return None


def parse_ssh_config(raw: str) -> dict[str, str]:
    config: dict[str, str] = {}
    for line in raw.splitlines():
        line = line.strip()
        if not line or line.lower().startswith("host "):
            continue
        pieces = line.split(None, 1)
        if len(pieces) != 2:
            continue
        key = pieces[0].lower()
        value = pieces[1].strip()
        if value.startswith('"') and value.endswith('"'):
            value = value[1:-1]
        if key == "identityfile" and key in config:
            continue
        config[key] = value

    required = {"hostname", "user", "port", "identityfile"}
    missing = required.difference(config)
    if missing:
        raise RuntimeError(f"Incomplete ssh-config ({missing}) in output:\n{raw}")
    return config


@lru_cache(maxsize=8)
def _ssh_config(machine: str) -> str:
    return subprocess.check_output(
        ["vagrant", "ssh-config", machine],
        cwd=VAGRANT_DIR,
        text=True,
    )


def get_ssh_host(machine: str) -> Host:
    ensure_machine_running(machine)
    raw = _ssh_config(machine)
    config = parse_ssh_config(raw)
    return testinfra.get_host(
        f"paramiko://{config['user']}@{config['hostname']}:{config['port']}",
        ssh_identity_file=config["identityfile"],
    )


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


def run_powershell(host: Host, script: str) -> CommandResult:
    encoded = encode_powershell(script)
    return host.run(f"powershell -NoProfile -EncodedCommand {encoded}")


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def run_guest_script(host: Host, relative_path: str, **parameters: object) -> CommandResult:
    script_path = GUEST_TESTS_ROOT / relative_path
    ps_path = str(script_path).replace("'", "''")
    args = "".join(f" -{key} {_ps_quote(str(value))}" for key, value in parameters.items())
    command = (
        "powershell -NoProfile -ExecutionPolicy Bypass "
        f"-Command \"& '{ps_path}'{args}\""
    )
    return host.run(command)


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped[len(marker) :].strip()
    return None


def run_xp2p(host: Host, args: Iterable[str]) -> CommandResult:
    xp2p_ps = str(XP2P_EXE).replace("\\", "\\\\")
    arguments = ", ".join(ps_quote(str(arg)) for arg in args)
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{xp2p_ps}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}
$arguments = @({arguments})
& $xp2p @arguments
exit $LASTEXITCODE
"""
    return run_powershell(host, script)


def ensure_msi_package(host: Host) -> str:
    """Builds the MSI inside the guest if it does not exist and returns its path."""
    global _MSI_CACHE_PATH
    if _MSI_CACHE_PATH:
        return _MSI_CACHE_PATH

    script = f"""
$ErrorActionPreference = 'Stop'
$repo = 'C:\\xp2p'
$cacheDir = 'C:\\xp2p\\build\\msi-cache'
$marker = '{MSI_MARKER}'
if (-not (Test-Path $cacheDir)) {{
    New-Item -ItemType Directory -Path $cacheDir -Force | Out-Null
}}
    Push-Location $repo
    try {{
        $version = (& go run ./go/cmd/xp2p --version).Trim()
        if ([string]::IsNullOrWhiteSpace($version)) {{
            throw "xp2p --version returned empty output."
        }}
        $headFile = Join-Path $repo '.git\\HEAD'
        if (-not (Test-Path $headFile)) {{
            throw ".git\\HEAD not found at $headFile"
        }}
        $headContent = (Get-Content -Raw $headFile).Trim()
        $commit = ''
        if ($headContent -like 'ref:*') {{
            $ref = $headContent.Substring(5).Trim()
            $refFile = Join-Path $repo ('.git\\' + $ref)
            if (Test-Path $refFile) {{
                $commit = (Get-Content -Raw $refFile).Trim()
            }} else {{
                $packedRefs = Join-Path $repo '.git\\packed-refs'
                if (Test-Path $packedRefs) {{
                    $match = Select-String -Path $packedRefs -Pattern ([regex]::Escape($ref)) | Select-Object -First 1
                    if ($match) {{
                        $commit = ($match.Line.Split(' '))[0].Trim()
                    }}
                }}
            }}
        }} else {{
            $commit = $headContent
        }}
        if ([string]::IsNullOrWhiteSpace($commit)) {{
            throw "Unable to resolve git commit hash."
        }}
        $dirty = ''
        try {{
            $gitCmd = (Get-Command git -ErrorAction Stop).Source
            $dirty = (& $gitCmd status --porcelain 2>$null).Trim()
        }} catch {{
            $dirty = ''
        }}
        if ($dirty) {{
            $commit = "$commit-dirty"
        }}
        $msiName = "xp2p-$version-$commit-windows-amd64.msi"
        $msiPath = Join-Path $cacheDir $msiName
        if (-not (Test-Path $msiPath)) {{
            $ldflags = "-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=$version"
            $binaryDir = Join-Path $repo 'build\\msi-bin'
            if (Test-Path $binaryDir) {{
            Remove-Item $binaryDir -Recurse -Force -ErrorAction SilentlyContinue
        }}
        New-Item -ItemType Directory -Path $binaryDir -Force | Out-Null
        $binaryPath = Join-Path $binaryDir 'xp2p.exe'
        & go build -trimpath -ldflags $ldflags -o $binaryPath ./go/cmd/xp2p | Write-Output
        if ($LASTEXITCODE -ne 0) {{
            throw "go build failed with exit code $LASTEXITCODE"
        }}
        $wixDir = Get-ChildItem "C:\\Program Files (x86)" -Filter "WiX Toolset*" -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1
        if (-not $wixDir) {{
            throw "WiX Toolset installation directory not found."
        }}
        $candle = Join-Path $wixDir.FullName 'bin\\candle.exe'
        $light = Join-Path $wixDir.FullName 'bin\\light.exe'
        $wixSource = Join-Path $repo 'installer\\wix\\xp2p.wxs'
        $wixObj = Join-Path $binaryDir 'xp2p.wixobj'
        & $candle "-dProductVersion=$version" "-dXp2pBinary=$binaryPath" "-out" $wixObj $wixSource | Write-Output
        if ($LASTEXITCODE -ne 0) {{
            throw "candle.exe failed with exit code $LASTEXITCODE"
        }}
        & $light "-out" $msiPath $wixObj | Write-Output
        if ($LASTEXITCODE -ne 0) {{
            throw "light.exe failed with exit code $LASTEXITCODE"
        }}
    }}
    Write-Output ("$marker$msiPath")
}}
finally {{
    Pop-Location
}}
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to build MSI package.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    path = _extract_marker(result.stdout, MSI_MARKER)
    if not path:
        raise RuntimeError(
            "MSI build script did not return artifact path.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    _MSI_CACHE_PATH = path
    return path


def install_xp2p_from_msi(host: Host, msi_path: str | Path) -> None:
    msi_str = ps_quote(str(msi_path))
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
if (-not (Test-Path $msi)) {{
    throw "MSI package not found at $msi"
}}
$arguments = @('/i', $msi, '/qn', '/norestart')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -Wait -PassThru
if ($process.ExitCode -ne 0) {{
    exit $process.ExitCode
}}
exit 0
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to install xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def get_remote_file_size(host: Host, path: str | Path) -> int:
    target = ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
if (-not (Test-Path $target)) {{
    throw "File not found at $target"
}}
$item = Get-Item $target
Write-Output $item.Length
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query remote file size.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        return int((result.stdout or "").strip().splitlines()[-1])
    except (ValueError, IndexError) as exc:
        raise RuntimeError(f"Unexpected size output: {result.stdout!r}") from exc
