import base64
import json
import uuid
from pathlib import Path
from typing import Iterable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "windows10"
VAGRANT_BUILDER_DIR = REPO_ROOT / "infra" / "vagrant" / "win-msi"
DEFAULT_SERVER = "win10-a"
DEFAULT_CLIENT = "win10-b"
BUILDER_MACHINE = "win-msi"
PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
LOGS_DIR = PROGRAM_FILES_INSTALL_DIR / "logs"
XP2P_EXE = PROGRAM_FILES_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60
GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")
LOCAL_GUEST_TESTS_ROOT = REPO_ROOT / "tests" / "guest"
MSI_MARKER = "__MSI_PATH__="

MSI_ARTIFACTS_DIR_X64 = Path(r"C:\xp2p\build\msi-artifacts")
MSI_ARTIFACTS_DIR_X86 = Path(r"C:\xp2p\build\msi-artifacts-x86")
MSI_LATEST_FILENAME = "latest.txt"
MSI_LATEST_PATH_X64 = MSI_ARTIFACTS_DIR_X64 / MSI_LATEST_FILENAME
MSI_LATEST_PATH_X86 = MSI_ARTIFACTS_DIR_X86 / MSI_LATEST_FILENAME

WIN_STACKS = {
    "win10": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "windows10",
        "server": "win10-a",
        "client": "win10-b",
    },
    "win2022": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "server2022",
        "server": "win2022-a",
        "client": "win2022-b",
    },
}

_CURRENT_WIN_STACK = "win10"


def available_win_stacks() -> list[str]:
    return sorted(WIN_STACKS.keys())


def set_win_stack(name: str) -> None:
    global _CURRENT_WIN_STACK, VAGRANT_DIR, DEFAULT_SERVER, DEFAULT_CLIENT
    if name not in WIN_STACKS:
        raise ValueError(
            f"Unknown win stack '{name}'. Available: {', '.join(available_win_stacks())}"
        )
    config = WIN_STACKS[name]
    _CURRENT_WIN_STACK = name
    VAGRANT_DIR = config["vagrant_dir"]
    DEFAULT_SERVER = config["server"]
    DEFAULT_CLIENT = config["client"]


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def require_builder_environment() -> None:
    common.require_vagrant_environment(VAGRANT_BUILDER_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def ensure_builder_running() -> None:
    common.ensure_machine_running(VAGRANT_BUILDER_DIR, BUILDER_MACHINE)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


def get_builder_host() -> Host:
    return common.get_ssh_host(VAGRANT_BUILDER_DIR, BUILDER_MACHINE)


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


def run_powershell(host: Host, script: str) -> CommandResult:
    encoded = encode_powershell(script)
    return host.run(f"powershell -NoProfile -EncodedCommand {encoded}")


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _stage_guest_script(host: Host, relative: Path, *, relative_label: str) -> tuple[Path, Path]:
    local_script = LOCAL_GUEST_TESTS_ROOT / relative
    if not local_script.exists():
        raise RuntimeError(f"Guest script {relative_label} not found at {local_script}")
    staged_path = Path(r"C:\Windows\Temp") / f"xp2p-guest-script-{uuid.uuid4().hex}.ps1"
    write_text(host, staged_path, local_script.read_text(encoding="utf-8"))
    return staged_path, staged_path


def _missing_script_error(result: CommandResult, script_path: Path) -> bool:
    combined = f"{result.stdout}\n{result.stderr}".lower()
    if not combined:
        return False
    target = str(script_path).lower()
    if target not in combined:
        return False
    indicators = (
        "commandnotfoundexception",
        "not recognized as the name of a cmdlet",
        "cannot find path",
    )
    return any(marker in combined for marker in indicators)


def run_guest_script(host: Host, relative_path: str, **parameters: object) -> CommandResult:
    relative = Path(relative_path)
    script_path = GUEST_TESTS_ROOT / relative
    cleanup_path: Path | None = None
    if not path_exists(host, script_path):
        script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)

    def _invoke(target: Path) -> CommandResult:
        ps_path = str(target).replace("'", "''")
        args = "".join(f" -{key} {_ps_quote(str(value))}" for key, value in parameters.items())
        command = (
            "powershell -NoProfile -ExecutionPolicy Bypass "
            f"-File \"{ps_path}\"{args}"
        )
        return host.run(command)

    try:
        result = _invoke(script_path)
        if result.rc != 0 and cleanup_path is None and _missing_script_error(result, script_path):
            script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
            result = _invoke(script_path)
        return result
    finally:
        if cleanup_path is not None:
            remove_path(host, cleanup_path)


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped[len(marker):].strip()
    return None


def parse_latest_content(content: str) -> dict[str, str]:
    data: dict[str, str] = {}
    for raw_line in (content or "").splitlines():
        line = raw_line.strip()
        if not line:
            continue
        parts = line.split("=", 1)
        if len(parts) != 2:
            raise RuntimeError(f"Invalid latest.txt line: {raw_line!r}")
        key = parts[0].strip().lower()
        value = parts[1].strip()
        data[key] = value
    required = {"version", "sha256", "msi_path"}
    missing = required.difference(data)
    if missing:
        raise RuntimeError(f"latest.txt missing keys: {sorted(missing)}")
    return data


def read_msi_latest(host: Host, latest_path: str | Path) -> dict[str, str]:
    content = read_text(host, latest_path)
    return parse_latest_content(content)


def get_msi_path_from_latest(host: Host, latest_path: str | Path) -> str:
    info = read_msi_latest(host, latest_path)
    return info["msi_path"]


def run_xp2p(host: Host, args: Iterable[str]) -> CommandResult:
    payload = _encode_args_payload(args)
    return run_guest_script(
        host,
        "scripts/run_xp2p_command.ps1",
        Xp2pPath=str(XP2P_EXE),
        ArgsBase64=payload,
    )


def _encode_args_payload(args: Iterable[str]) -> str:
    raw = json.dumps([str(arg) for arg in args])
    return base64.b64encode(raw.encode("utf-8")).decode("ascii")


def _ensure_log_directories(host: Host) -> None:
    log_dirs = [
        LOGS_DIR,
        LOGS_DIR / "client",
        LOGS_DIR / "server",
    ]
    dir_list = ", ".join(ps_quote(str(path)) for path in log_dirs)
    script = f"""
$ErrorActionPreference = 'Stop'
foreach ($dir in @({dir_list})) {{
    if (-not (Test-Path $dir)) {{
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }}
}}
"""
    run_powershell(host, script)


def ensure_msi_package(host: Host) -> str:
    _build_msi_package(
        host,
        architecture="amd64",
        cache_dir=MSI_ARTIFACTS_DIR_X64,
        wix_source=r"installer\wix\xp2p.wxs",
    )
    return str(MSI_LATEST_PATH_X64)


def ensure_msi_package_x86(host: Host) -> str:
    _build_msi_package(
        host,
        architecture="x86",
        cache_dir=MSI_ARTIFACTS_DIR_X86,
        wix_source=r"installer\wix\xp2p-x86.wxs",
    )
    return str(MSI_LATEST_PATH_X86)


def install_xp2p_from_latest(host: Host, latest_path: str | Path, *, force: bool = False) -> None:
    result = run_guest_script(
        host,
        "scripts/install_msi.ps1",
        LatestPath=str(latest_path),
        Force=str(force).lower(),
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to install xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def uninstall_xp2p_from_msi(host: Host, msi_path: str | Path) -> None:
    msi_str = ps_quote(str(msi_path))
    install_dir = ps_quote(str(PROGRAM_FILES_INSTALL_DIR))
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
$arguments = @('/x', $msi, '/qn', '/norestart')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
if (-not $process.WaitForExit(300000)) {{
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    exit 124
}}
if ($process.ExitCode -ne 0) {{
    exit $process.ExitCode
}}
if (Test-Path {install_dir}) {{
    Remove-Item {install_dir} -Force -Recurse -ErrorAction SilentlyContinue
}}
exit 0
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to uninstall xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def ensure_program_files_install(host: Host, *, force_reinstall: bool = False) -> None:
    if not force_reinstall and path_exists(host, XP2P_EXE):
        _ensure_log_directories(host)
        return

    install_xp2p_from_latest(host, MSI_LATEST_PATH_X64, force=force_reinstall)

    if not path_exists(host, XP2P_EXE):
        raise RuntimeError(
            f"xp2p.exe not found at {XP2P_EXE} after MSI installation on remote host."
        )
    _ensure_log_directories(host)


def ensure_admin_token(host: Host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
$path = 'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System'
$value = Get-ItemProperty -Path $path -Name 'LocalAccountTokenFilterPolicy' -ErrorAction SilentlyContinue
if (-not $value -or $value.LocalAccountTokenFilterPolicy -ne 1) {
    New-ItemProperty -Path $path -Name 'LocalAccountTokenFilterPolicy' -PropertyType DWord -Value 1 -Force | Out-Null
}
"""
    run_powershell(host, script)


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


def path_exists(host: Host, path: Path | str) -> bool:
    target = ps_quote(str(path))
    script = f"if (Test-Path {target}) {{ exit 0 }} else {{ exit 3 }}"
    result = run_powershell(host, script)
    return result.rc == 0


def remove_path(host: Host, path: Path | str) -> None:
    target = ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (Test-Path {target}) {{
    Remove-Item {target} -Force -Recurse -ErrorAction SilentlyContinue
}}
"""
    run_powershell(host, script)


def read_text(host: Host, path: Path | str) -> str:
    target = ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {target})) {{
    exit 3
}}
Get-Content -Raw {target}
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def write_text(host: Host, path: Path | str, content: str) -> None:
    target = ps_quote(str(path))
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$data = [System.Convert]::FromBase64String('{encoded}')
[System.IO.File]::WriteAllBytes($target, $data)
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
