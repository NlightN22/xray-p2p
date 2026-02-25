import base64
import hashlib
import json
import os
import time
import uuid
from pathlib import Path
from typing import Iterable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

try:
    import tomllib
except ImportError:  # pragma: no cover - fallback for older runtimes.
    import tomli as tomllib

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "windows10"
DEFAULT_SERVER = "win10-a"
DEFAULT_CLIENT = "win10-b"
DEFAULT_TARGET = "10.62.10.21"
PROGRAM_FILES_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
PROGRAM_FILES_X86_INSTALL_DIR = Path(r"C:\Program Files (x86)\xp2p")
PROGRAM_DATA_ROOT = Path(os.environ.get("ProgramData", r"C:\ProgramData")) / "xp2p"
CONFIG_ROOT = Path(os.environ.get("XP2P_CONFIG_ROOT", str(PROGRAM_DATA_ROOT)))
LOGS_DIR = Path(os.environ.get("XP2P_LOG_ROOT", str(CONFIG_ROOT / "logs")))
XP2P_EXE = PROGRAM_FILES_INSTALL_DIR / "xp2p.exe"
SERVICE_START_TIMEOUT = 60
GUEST_TESTS_ROOT = Path(r"C:\xp2p\tests\guest")
LOCAL_GUEST_TESTS_ROOT = REPO_ROOT / "tests" / "guest"
MSI_MARKER = "__MSI_PATH__="

MSI_CACHE_DIR_X64 = Path(r"C:\xp2p\build\msi-cache")
MSI_CACHE_DIR_X86 = Path(r"C:\xp2p\build\msi-cache-x86")
PROJECT_SYNC_MARKER = Path(r"C:\xp2p\scripts\build\build_and_install_msi.ps1")

_MSI_CACHE_PATH_X64: str | None = None
_MSI_CACHE_PATH_X86: str | None = None
_MSI_BUILD_ID: str | None = None

WIN_STACKS = {
    "win7": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "windows7",
        "server": "win7-a",
        "client": "win7-b",
        "target": "10.62.10.61",
    },
    "win10": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "windows10",
        "server": "win10-a",
        "client": "win10-b",
        "target": "10.62.10.21",
    },
    "win2016": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "server2016",
        "server": "win2016-a",
        "client": "win2016-b",
        "target": "10.62.10.51",
    },
    "win2022": {
        "vagrant_dir": REPO_ROOT / "infra" / "vagrant" / "server2022",
        "server": "win2022-a",
        "client": "win2022-b",
        "target": "10.62.10.31",
    },
}

_CURRENT_WIN_STACK = "win10"


def available_win_stacks() -> list[str]:
    return sorted(WIN_STACKS.keys())


def set_win_stack(name: str) -> None:
    global _CURRENT_WIN_STACK, VAGRANT_DIR, DEFAULT_SERVER, DEFAULT_CLIENT, DEFAULT_TARGET
    if name not in WIN_STACKS:
        raise ValueError(
            f"Unknown win stack '{name}'. Available: {', '.join(available_win_stacks())}"
        )
    config = WIN_STACKS[name]
    _CURRENT_WIN_STACK = name
    VAGRANT_DIR = config["vagrant_dir"]
    DEFAULT_SERVER = config["server"]
    DEFAULT_CLIENT = config["client"]
    DEFAULT_TARGET = config["target"]


def set_msi_build_id(build_id: str | None) -> None:
    global _MSI_BUILD_ID
    _MSI_BUILD_ID = build_id


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


DEFAULT_POWERSHELL_TIMEOUT = 120
DEFAULT_GUEST_SCRIPT_TIMEOUT = 900


def run_powershell(host: Host, script: str, timeout: int | float | None = None) -> CommandResult:
    encoded = encode_powershell(script)
    started = time.monotonic()
    effective_timeout = DEFAULT_POWERSHELL_TIMEOUT if timeout is None else timeout
    result = host.run(
        f"powershell -NoProfile -NonInteractive -NoLogo -EncodedCommand {encoded}",
        timeout=effective_timeout,
    )
    elapsed_ms = int((time.monotonic() - started) * 1000)
    if elapsed_ms > 2000:
        print(f"TIMING: powershell_ms={elapsed_ms}")
    return result


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _sha256_bytes(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def _remote_sha256(host: Host, path: Path) -> str | None:
    target = ps_quote(str(path))
    script = (
        f"if (Test-Path {target}) {{ "
        f"(Get-FileHash -Algorithm SHA256 -Path {target}).Hash "
        "}"
    )
    result = run_powershell(host, script)
    if result.rc != 0:
        return None
    return (result.stdout or "").strip() or None


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


def run_guest_script(
    host: Host,
    relative_path: str,
    *,
    force_stage: bool = False,
    timeout: int | float | None = None,
    **parameters: object,
) -> CommandResult:
    start = time.perf_counter()
    relative = Path(relative_path)
    script_path = GUEST_TESTS_ROOT / relative
    cleanup_path: Path | None = None
    local_script = LOCAL_GUEST_TESTS_ROOT / relative
    local_hash = _sha256_bytes(local_script.read_bytes())
    if force_stage or not path_exists(host, script_path):
        script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
    else:
        remote_hash = _remote_sha256(host, script_path)
        if not remote_hash or remote_hash.lower() != local_hash.lower():
            script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)

    def _invoke(target: Path) -> CommandResult:
        ps_path = str(target).replace('"', '""')
        args = "".join(f" -{key} {_ps_quote(str(value))}" for key, value in parameters.items())
        command = (
            "powershell -NoProfile -ExecutionPolicy Bypass "
            f"-File \"{ps_path}\"{args}"
        )
        effective_timeout = DEFAULT_GUEST_SCRIPT_TIMEOUT if timeout is None else timeout
        return host.run(command, timeout=effective_timeout)

    try:
        result = _invoke(script_path)
        if result.rc != 0 and cleanup_path is None and _missing_script_error(result, script_path):
            script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
            result = _invoke(script_path)
        return result
    finally:
        elapsed = time.perf_counter() - start
        if elapsed > 2.0:
            print(f"TIMING: run_guest_script {relative_path}: {elapsed:.2f}s")
        if cleanup_path is not None:
            remove_path(host, cleanup_path)


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped[len(marker):].strip()
    return None


def run_xp2p(host: Host, args: Iterable[str]) -> CommandResult:
    payload = _encode_args_payload(args)
    result = run_guest_script(
        host,
        "scripts/run_xp2p_command.ps1",
        Xp2pPath=str(XP2P_EXE),
        ArgsBase64=payload,
    )
    for line in (result.stdout or "").splitlines():
        if line.startswith("TIMING:"):
            print(line)
    return result


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
    global _MSI_CACHE_PATH_X64
    if _MSI_CACHE_PATH_X64 and path_exists(host, _MSI_CACHE_PATH_X64):
        return _MSI_CACHE_PATH_X64

    ensure_project_synced(host)
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
    if _MSI_CACHE_PATH_X86 and path_exists(host, _MSI_CACHE_PATH_X86):
        return _MSI_CACHE_PATH_X86

    ensure_project_synced(host)
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
$arguments = @('/i', $msi, '/qn', '/norestart', 'XP2P_SKIP_SERVICE_START=1')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
if (-not $process.WaitForExit(300000)) {{
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    exit 124
}}
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


def uninstall_xp2p_from_msi(host: Host, msi_path: str | Path, *, purge_files: bool = True) -> None:
    msi_str = ps_quote(str(msi_path))
    install_dir = ps_quote(str(get_program_files_install_dir(host)))
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
$services = @('xp2p-client', 'xp2p-server')
foreach ($svc in $services) {{
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service -and $service.Status -ne 'Stopped') {{
        Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
    }}
}}
Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
$arguments = @('/x', $msi, '/qn', '/norestart')
$attempt = 0
do {{
    $attempt++
    $process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
    if (-not $process.WaitForExit(300000)) {{
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        exit 124
    }}
    $successCodes = @(0, 1605, 1614, 3010)
    if ($successCodes -contains $process.ExitCode) {{
        break
    }}
    Start-Sleep -Seconds 2
}} while ($attempt -lt 2)
if ($successCodes -notcontains $process.ExitCode) {{
    exit $process.ExitCode
}}
"""
    if purge_files:
        script += f"""
if (Test-Path {install_dir}) {{
    Remove-Item {install_dir} -Force -Recurse -ErrorAction SilentlyContinue
}}
"""
    script += """
exit 0
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to uninstall xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def ensure_program_files_install(host: Host, *, force_reinstall: bool = False) -> None:
    if not force_reinstall:
        detected = _detect_xp2p_exe(host)
        if detected is not None:
            _set_install_paths_from_exe(detected)
            _ensure_log_directories(host)
            return

    start = time.perf_counter()
    msi_path = ensure_msi_package(host)
    print(f"TIMING: ensure_msi_package: {time.perf_counter() - start:.2f}s")
    start = time.perf_counter()
    install_xp2p_from_msi(host, msi_path)
    print(f"TIMING: install_xp2p_from_msi: {time.perf_counter() - start:.2f}s")

    start = time.perf_counter()
    detected = _detect_xp2p_exe(host)
    print(f"TIMING: detect_xp2p_exe: {time.perf_counter() - start:.2f}s")
    if detected is None:
        raise RuntimeError(
            "xp2p.exe not found after MSI installation on remote host. "
            f"Checked: {PROGRAM_FILES_INSTALL_DIR} and {PROGRAM_FILES_X86_INSTALL_DIR}."
        )
    _set_install_paths_from_exe(detected)
    _ensure_log_directories(host)


def get_program_files_install_dir(host: Host) -> Path:
    detected = _detect_xp2p_exe(host)
    if detected is not None:
        return _set_install_paths_from_exe(detected)
    return PROGRAM_FILES_INSTALL_DIR


def _set_install_paths_from_exe(exe_path: Path) -> Path:
    global PROGRAM_FILES_INSTALL_DIR, CONFIG_ROOT, LOGS_DIR, XP2P_EXE
    install_dir = exe_path.parent
    if install_dir.name.lower() == "bin" and install_dir.parent:
        install_dir = install_dir.parent
    PROGRAM_FILES_INSTALL_DIR = install_dir
    if "XP2P_CONFIG_ROOT" not in os.environ:
        CONFIG_ROOT = PROGRAM_DATA_ROOT
    if "XP2P_LOG_ROOT" not in os.environ:
        LOGS_DIR = CONFIG_ROOT / "logs"
    XP2P_EXE = exe_path
    return PROGRAM_FILES_INSTALL_DIR


def _detect_xp2p_exe(host: Host) -> Path | None:
    candidates = [
        PROGRAM_FILES_INSTALL_DIR / "xp2p.exe",
        PROGRAM_FILES_INSTALL_DIR / "bin" / "xp2p.exe",
        PROGRAM_FILES_X86_INSTALL_DIR / "xp2p.exe",
        PROGRAM_FILES_X86_INSTALL_DIR / "bin" / "xp2p.exe",
    ]
    for candidate in candidates:
        if path_exists(host, candidate):
            return candidate

    install_root = _query_install_location(host)
    if install_root:
        for candidate in (
            install_root / "xp2p.exe",
            install_root / "bin" / "xp2p.exe",
        ):
            if path_exists(host, candidate):
                return candidate

    search_roots = [
        Path(r"C:\Program Files"),
        Path(r"C:\Program Files (x86)"),
        Path(r"C:\ProgramData"),
    ]
    roots = ", ".join(ps_quote(str(root)) for root in search_roots)
    script = f"""
$ErrorActionPreference = 'Stop'
$roots = @({roots})
foreach ($root in $roots) {{
    if (-not (Test-Path $root)) {{
        continue
    }}
    $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if ($found) {{
        Write-Output $found
        exit 0
    }}
}}
exit 3
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        return _search_user_programs(host)
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return _search_user_programs(host)
    return Path(value[-1].strip())


def _search_user_programs(host: Host) -> Path | None:
    script = """
$ErrorActionPreference = 'Stop'
$usersRoot = 'C:\\Users'
if (-not (Test-Path $usersRoot)) {
    exit 3
}
$users = Get-ChildItem -Path $usersRoot -Directory -ErrorAction SilentlyContinue
foreach ($user in $users) {
    $root = Join-Path $user.FullName 'AppData\\Local\\Programs'
    if (-not (Test-Path $root)) {
        continue
    }
    $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if ($found) {
        Write-Output $found
        exit 0
    }
}
exit 3
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


def _query_install_location(host: Host) -> Path | None:
    script = """
$ErrorActionPreference = 'SilentlyContinue'
$roots = @(
    'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKLM:\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*'
)
$items = Get-ItemProperty -Path $roots | Where-Object {
    $_.DisplayName -and $_.DisplayName -like 'xp2p*'
}
foreach ($item in $items) {
    if ($item.InstallLocation) {
        Write-Output $item.InstallLocation
        exit 0
    }
}
exit 3
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


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
        BuildId=_MSI_BUILD_ID or "",
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


def ensure_project_synced(
    host: Host,
    *,
    timeout: int = 60,
    machine: str | None = None,
) -> None:
    if _wait_for_sync_marker(host, timeout=timeout):
        return
    hint = ""
    if machine:
        hint = f" Re-mount the synced folder (try 'vagrant reload --provision {machine}')."
    raise RuntimeError(
        "Project sync not ready on guest. "
        f"Expected {PROJECT_SYNC_MARKER} to exist.{hint}"
    )


def _wait_for_sync_marker(host: Host, *, timeout: int) -> bool:
    start = time.monotonic()
    while time.monotonic() - start < timeout:
        if path_exists(host, PROJECT_SYNC_MARKER):
            return True
        time.sleep(2)
    return False


def get_host_ipv4(host: Host) -> str:
    result = run_guest_script(
        host,
        "scripts/get_ipv4_addresses.ps1",
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to detect IPv4 addresses.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        raise RuntimeError("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]




def remove_paths(host: Host, paths: Iterable[Path | str]) -> None:
    targets = [ps_quote(str(path)) for path in paths]
    if not targets:
        return
    target_list = ", ".join(targets)
    script = f"""
$ErrorActionPreference = 'Stop'
foreach ($target in @({target_list})) {{
    if (Test-Path $target) {{
        Remove-Item $target -Force -Recurse -ErrorAction SilentlyContinue
    }}
}}
"""
    run_powershell(host, script)


def cleanup_xp2p_install(
    host: Host,
    *,
    config_dirs: Iterable[Path],
    state_files: Iterable[Path],
    extra_paths: Iterable[Path] = (),
) -> None:
    remove_paths(
        host,
        [*config_dirs, *state_files, *extra_paths],
    )


def paths_exist(host: Host, paths: Iterable[Path | str]) -> set[str]:
    targets = [ps_quote(str(path)) for path in paths]
    if not targets:
        return set()
    target_list = ", ".join(targets)
    script = f"""
$ErrorActionPreference = 'Stop'
$existing = @()
foreach ($target in @({target_list})) {{
    if (Test-Path $target) {{
        $existing += $target
    }}
}}
$existing
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to check remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return {line.strip().strip("'") for line in (result.stdout or "").splitlines() if line.strip()}


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


def read_toml(host: Host, path: Path | str) -> dict:
    content = read_text(host, path)
    try:
        return tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


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
