import base64
from pathlib import Path
from typing import Iterable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "windows10"
DEFAULT_SERVER = "win10-server"
DEFAULT_CLIENT = "win10-client"
PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
XP2P_EXE = PROGRAM_FILES_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60
GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")
MSI_MARKER = "__MSI_PATH__="

MSI_CACHE_DIR_X64 = Path(r"C:\xp2p\build\msi-cache")
MSI_CACHE_DIR_X86 = Path(r"C:\xp2p\build\msi-cache-x86")

_MSI_CACHE_PATH_X64: str | None = None
_MSI_CACHE_PATH_X86: str | None = None


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


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
            return stripped[len(marker):].strip()
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
    global _MSI_CACHE_PATH_X64
    if _MSI_CACHE_PATH_X64:
        return _MSI_CACHE_PATH_X64

    path = _build_msi_package(
        host,
        architecture="amd64",
        cache_dir=MSI_CACHE_DIR_X64,
        wix_source=r"installer\wix\xp2p.wxs",
    )
    _MSI_CACHE_PATH_X64 = path
    return path


def ensure_msi_package_x86(host: Host) -> str:
    global _MSI_CACHE_PATH_X86
    if _MSI_CACHE_PATH_X86:
        return _MSI_CACHE_PATH_X86

    path = _build_msi_package(
        host,
        architecture="x86",
        cache_dir=MSI_CACHE_DIR_X86,
        wix_source=r"installer\wix\xp2p-x86.wxs",
    )
    _MSI_CACHE_PATH_X86 = path
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


def _build_msi_package(
    host: Host,
    *,
    architecture: str,
    cache_dir: Path,
    wix_source: str,
) -> str:
    result = run_guest_script(
        host,
        "scripts/build_msi_package.ps1",
        Architecture=architecture,
        CacheDir=str(cache_dir),
        WixSource=wix_source,
        Marker=MSI_MARKER,
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to build MSI package for {architecture}.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    path = _extract_marker(result.stdout, MSI_MARKER)
    if not path:
        raise RuntimeError(
            f"MSI build script ({architecture}) did not return artifact path.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return path
