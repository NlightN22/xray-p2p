import base64
import hashlib
import json
import os
import time
import uuid
from pathlib import Path
from typing import Iterable
from concurrent.futures import ThreadPoolExecutor

import pytest
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
APPLY_DIR_NAME = ".state"
PENDING_DIR_NAME = "pending"
LIVE_DIR_NAME = "live"
LKG_DIR_NAME = "lkg"
CLIENT_CONFIG_DIR_NAME = "config-client"
SERVER_CONFIG_DIR_NAME = "config-server"
CLIENT_CONFIG_DIR = CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR = CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
CONFIG_PENDING_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / PENDING_DIR_NAME
CONFIG_LIVE_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / LIVE_DIR_NAME
CONFIG_LKG_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / LKG_DIR_NAME
CLIENT_PENDING_DIR = CONFIG_PENDING_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_PENDING_DIR = CONFIG_PENDING_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_LIVE_DIR = CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_LIVE_DIR = CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME
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
_GUEST_SCRIPT_CACHE: dict[tuple[str, str], str] = {}

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


class MsiServiceUnavailable(RuntimeError):
    pass


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
    probe_timeout = 30

    def _probe(host: Host) -> None:
        result = run_powershell(
            host,
            "[Environment]::MachineName; whoami",
            timeout=probe_timeout,
            label="ssh_probe",
        )
        if result.rc != 0:
            raise RuntimeError(
                "SSH probe command failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

    last_exc: BaseException | None = None
    for attempt in range(1, 3):
        try:
            host = common.get_ssh_host(VAGRANT_DIR, machine, connect_timeout=probe_timeout)
            setattr(host, "_xp2p_machine", machine)
            _probe(host)
            return host
        except pytest.skip.Exception as exc:
            last_exc = exc
        except Exception as exc:
            last_exc = exc

        if attempt == 1:
            print(
                f"WARNING: SSH probe failed for guest {machine} ({type(last_exc).__name__}). "
                "Reloading the VM and retrying once."
            )
            try:
                common.vagrant_reload_force(VAGRANT_DIR, machine)
            finally:
                common.invalidate_ssh_config_cache()
            time.sleep(5)

    if last_exc is None:
        raise RuntimeError(f"Failed to connect to guest {machine} via SSH.")
    raise last_exc


def _refresh_ssh_host(host: Host, *, connect_timeout: int) -> Host | None:
    machine = getattr(host, "_xp2p_machine", None)
    if not machine:
        return None
    try:
        refreshed = common.get_ssh_host(VAGRANT_DIR, machine, connect_timeout=connect_timeout)
    except Exception:
        return None
    setattr(refreshed, "_xp2p_machine", machine)
    return refreshed


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


DEFAULT_POWERSHELL_TIMEOUT = 120
DEFAULT_GUEST_SCRIPT_TIMEOUT = 120
DEFAULT_XP2P_COMMAND_TIMEOUT = 120
ADMIN_XP2P_SUBCOMMANDS = {"run", "service"}
GUEST_BUILD_ROOT = Path(r"C:\xp2p\build")


def _ssh_run_with_refresh(
    host: Host,
    command: str,
    *,
    timeout: int | float,
    label: str | None,
) -> CommandResult:
    try:
        return host.run(command, timeout=timeout)
    except pytest.skip.Exception as exc:
        refreshed = _refresh_ssh_host(host, connect_timeout=30)
        if refreshed is None:
            raise
        print(
            f"WARNING: SSH command skipped, retrying once with refreshed connection (label={label or 'n/a'})."
        )
        try:
            return refreshed.run(command, timeout=timeout)
        except pytest.skip.Exception:
            raise exc


def run_powershell(
    host: Host,
    script: str,
    timeout: int | float | None = None,
    *,
    label: str | None = None,
) -> CommandResult:
    encoded = encode_powershell(script)
    started = time.monotonic()
    effective_timeout = DEFAULT_POWERSHELL_TIMEOUT if timeout is None else timeout
    print(f"Guest PowerShell start (timeout={effective_timeout}s, label={label or 'n/a'})")
    lines = [line.rstrip() for line in script.strip().splitlines() if line.strip()]
    if not lines:
        summary = "<empty>"
    else:
        summary = lines[0].strip()
        if len(summary) > 160:
            summary = summary[:157] + "..."
        if len(lines) > 1:
            summary = f"{summary} (+{len(lines) - 1} lines)"
    print(f"Guest PowerShell script summary: {summary}")
    result = _ssh_run_with_refresh(
        host,
        f"powershell -NoProfile -NonInteractive -NoLogo -EncodedCommand {encoded}",
        timeout=effective_timeout,
        label=label,
    )
    elapsed_ms = int((time.monotonic() - started) * 1000)
    if elapsed_ms > 2000:
        if label:
            print(f"TIMING: powershell_ms={elapsed_ms} label={label}")
        else:
            print(f"TIMING: powershell_ms={elapsed_ms}")
    print(f"Guest PowerShell done (rc={result.rc})")
    return result


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def _sha256_bytes(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def _guest_script_cache_key(host: Host, script_path: Path) -> tuple[str, str]:
    backend = getattr(host, "backend", None)
    host_id = None
    if backend is not None:
        host_id = getattr(backend, "host", None) or getattr(backend, "hostname", None)
        port = getattr(backend, "port", None)
        if host_id is not None and port is not None:
            host_id = f"{host_id}:{port}"
    if host_id is None:
        host_id = repr(host)
    else:
        host_id = str(host_id)
    return host_id, str(script_path)


def _remote_sha256(host: Host, path: Path) -> str | None:
    target = ps_quote(str(path))
    script = (
        f"if (Test-Path {target}) {{ "
        f"(Get-FileHash -Algorithm SHA256 -Path {target}).Hash "
        "}"
    )
    result = run_powershell(host, script, label="remote_sha256")
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
    cache_key = _guest_script_cache_key(host, script_path)
    cached_hash = _GUEST_SCRIPT_CACHE.get(cache_key)
    use_cached = bool(cached_hash and cached_hash.lower() == local_hash.lower() and not force_stage)
    if not use_cached:
        if force_stage or not _path_exists_raw(host, script_path):
            script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
        else:
            remote_hash = _remote_sha256(host, script_path)
            if not remote_hash or remote_hash.lower() != local_hash.lower():
                script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
            else:
                _GUEST_SCRIPT_CACHE[cache_key] = local_hash

    def _invoke(target: Path) -> CommandResult:
        ps_path = str(target).replace('"', '""')
        args = "".join(f" -{key} {_ps_quote(str(value))}" for key, value in parameters.items())
        command = (
            "powershell -NoProfile -ExecutionPolicy Bypass "
            f"-File \"{ps_path}\"{args}"
        )
        effective_timeout = DEFAULT_GUEST_SCRIPT_TIMEOUT if timeout is None else timeout
        print(f"Guest script start: {relative_path} (timeout={effective_timeout}s)")
        if parameters:
            print(f"Guest script params: {sorted(parameters.keys())}")
        return _ssh_run_with_refresh(
            host,
            command,
            timeout=effective_timeout,
            label=f"guest_script:{relative_path}",
        )

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
        if 'result' in locals():
            print(f"Guest script done: {relative_path} (rc={result.rc})")
            if relative_path == "scripts/check_xp2p_ui_logs.ps1":
                stdout = (result.stdout or "").strip()
                stderr = (result.stderr or "").strip()
                if stdout:
                    print("Guest script stdout (check_xp2p_ui_logs.ps1):")
                    print(stdout)
                if stderr:
                    print("Guest script stderr (check_xp2p_ui_logs.ps1):")
                    print(stderr)
        if cleanup_path is not None:
            remove_path(host, cleanup_path)


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped[len(marker):].strip()
    return None


def run_xp2p(
    host: Host,
    args: Iterable[str],
    *,
    timeout: int | float | None = None,
) -> CommandResult:
    effective_timeout = DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout

    def _run(attempt_host: Host) -> CommandResult:
        timeout_marker, guest_timeout_marker = _xp2p_timeout_marker()
        if timeout_marker.exists():
            timeout_marker.unlink(missing_ok=True)
        executor = ThreadPoolExecutor(max_workers=1)
        future = executor.submit(
            _run_xp2p_with_timeout_marker, attempt_host, args, effective_timeout, guest_timeout_marker
        )
        deadline = time.monotonic() + float(effective_timeout)
        try:
            while True:
                if future.done():
                    result = future.result()
                    if timeout_marker.exists():
                        marker = timeout_marker.read_text(encoding="ascii", errors="ignore").strip()
                        raise RuntimeError(
                            "xp2p command timed out (marker observed after completion).\n"
                            f"Marker: {marker or '<empty>'}\nPath: {timeout_marker}"
                        )
                    return result
                if timeout_marker.exists():
                    marker = timeout_marker.read_text(encoding="ascii", errors="ignore").strip()
                    raise RuntimeError(
                        "xp2p command timed out while waiting for guest completion.\n"
                        f"Marker: {marker or '<empty>'}\nPath: {timeout_marker}"
                    )
                if time.monotonic() > deadline:
                    raise RuntimeError(
                        "xp2p command timed out before any guest marker appeared.\n"
                        f"Timeout marker path: {timeout_marker}"
                    )
                time.sleep(0.5)
        finally:
            executor.shutdown(wait=False)
            if future.done() and timeout_marker.exists():
                timeout_marker.unlink(missing_ok=True)

    try:
        return _run(host)
    except RuntimeError as exc:
        if "timed out before any guest marker appeared" not in str(exc):
            raise
        refreshed = _refresh_ssh_host(host, connect_timeout=30)
        if refreshed is None:
            raise
        print("WARNING: xp2p command timed out without guest markers; retrying once with refreshed SSH connection.")
        return _run(refreshed)


def _run_xp2p_admin(
    host: Host,
    args: Iterable[str],
    *,
    timeout: int | float | None = None,
    timeout_marker: Path | None = None,
) -> CommandResult:
    payload = _encode_args_payload(args)
    result = run_guest_script(
        host,
        "scripts/run_xp2p_command.ps1",
        Xp2pPath=str(XP2P_EXE),
        ArgsBase64=payload,
        TimeoutPath=str(timeout_marker) if timeout_marker else "",
        TimeoutSeconds=str(DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout),
        timeout=DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout,
    )
    stdout = result.stdout or ""
    if "__XP2P_TIMEOUT__" in stdout or "__XP2P_NOEXIT__" in stdout:
        raise RuntimeError(
            "xp2p scheduled task timed out in run_xp2p_command.ps1.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    for line in (result.stdout or "").splitlines():
        if line.startswith("TIMING:"):
            print(line)
    return result


def _run_xp2p_direct(
    host: Host,
    args: Iterable[str],
    *,
    timeout: int | float | None = None,
) -> CommandResult:
    arguments = [str(arg) for arg in args]
    arg_list = ", ".join(ps_quote(arg) for arg in arguments)
    if not arg_list:
        arg_list = ""
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = {ps_quote(str(XP2P_EXE))}
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}
$arguments = @({arg_list})
& $xp2p @arguments
exit $LASTEXITCODE
"""
    return run_powershell(
        host,
        script,
        timeout=DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout,
    )


def _xp2p_requires_admin(args: Iterable[str]) -> bool:
    parts = [str(arg).strip().lower() for arg in args if str(arg).strip()]
    if not parts:
        return False
    idx = 0
    while idx < len(parts) and parts[idx].startswith("-"):
        idx += 1
    if idx >= len(parts):
        return False
    if parts[idx] in {"client", "server"} and idx + 1 < len(parts):
        return parts[idx + 1] in ADMIN_XP2P_SUBCOMMANDS
    return False


def _xp2p_timeout_marker() -> tuple[Path, Path]:
    marker_dir = REPO_ROOT / "build" / "xp2p-timeouts"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = uuid.uuid4().hex
    local_path = marker_dir / f"xp2p-timeout-{token}.txt"
    guest_path = GUEST_BUILD_ROOT / "xp2p-timeouts" / f"xp2p-timeout-{token}.txt"
    return local_path, guest_path


def _admin_token_marker() -> tuple[Path, Path]:
    marker_dir = REPO_ROOT / "build" / "admin-token"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = uuid.uuid4().hex
    local_path = marker_dir / f"admin-token-{token}.txt"
    guest_path = Path(r"C:\Windows\Temp") / f"xp2p-admin-token-{token}.txt"
    return local_path, guest_path


def _msi_build_markers() -> tuple[Path, Path, Path]:
    marker_dir = REPO_ROOT / "build" / "msi-build"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = uuid.uuid4().hex
    local_path = marker_dir / f"msi-build-{token}.txt"
    guest_start = Path(r"C:\Windows\Temp") / f"xp2p-msi-build-start-{token}.txt"
    guest_done = Path(r"C:\Windows\Temp") / f"xp2p-msi-build-done-{token}.txt"
    return local_path, guest_start, guest_done


def _run_xp2p_with_timeout_marker(
    host: Host,
    args: Iterable[str],
    timeout: int | float,
    timeout_marker: Path,
) -> CommandResult:
    if _xp2p_requires_admin(args):
        return _run_xp2p_admin(host, args, timeout=timeout, timeout_marker=timeout_marker)
    return _run_xp2p_direct(host, args, timeout=timeout)


def _encode_args_payload(args: Iterable[str]) -> str:
    raw = json.dumps([str(arg) for arg in args])
    return base64.b64encode(raw.encode("utf-8")).decode("ascii")


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
    log_path = ps_quote(r"C:\xp2p\build\logs\win\msi-install.log")
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
if (-not (Test-Path $msi)) {{
    throw "MSI package not found at $msi"
}}
$policyRoots = @(
    'HKLM:\\Software\\Policies\\Microsoft\\Windows\\Installer',
    'HKCU:\\Software\\Policies\\Microsoft\\Windows\\Installer'
)
foreach ($policyRoot in $policyRoots) {{
    if (Test-Path $policyRoot) {{
        Set-ItemProperty -Path $policyRoot -Name 'DisableMSI' -Value 0 -ErrorAction SilentlyContinue
    }}
}}
$svc = Get-Service -Name 'msiserver' -ErrorAction SilentlyContinue
if ($svc) {{
    if ($svc.StartType -eq 'Disabled') {{
        Set-Service -Name 'msiserver' -StartupType Manual -ErrorAction SilentlyContinue
    }}
    if ($svc.Status -ne 'Running') {{
        Start-Service -Name 'msiserver' -ErrorAction SilentlyContinue
    }}
}}
$logPath = {log_path}
$logDir = Split-Path -Parent $logPath
if ($logDir -and -not (Test-Path $logDir)) {{
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}}
if (Test-Path $logPath) {{
    Remove-Item $logPath -Force -ErrorAction SilentlyContinue
}}
$arguments = @('/i', $msi, '/qn', '/norestart', '/l*v', $logPath, 'XP2P_SKIP_SERVICE_START=1')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
if (-not $process.WaitForExit(300000)) {{
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    exit 124
}}
if ($process.ExitCode -ne 0) {{
    Write-Output ("MSI ExitCode=" + $process.ExitCode)
    exit $process.ExitCode
}}
exit 0
"""
    result = run_powershell(host, script, label="path_exists_raw")
    if result.rc != 0:
        log_path = Path(r"C:\xp2p\build\logs\win\msi-install.log")
        log_tail = _read_msi_log_tail(host, log_path)
        log_context = _read_msi_failure_context(host, log_path)
        stdout = result.stdout or ""
        if "MSI ExitCode=1601" in stdout:
            run_powershell(
                host,
                """
$ErrorActionPreference = 'SilentlyContinue'
foreach ($policyRoot in @(
    'HKLM:\\Software\\Policies\\Microsoft\\Windows\\Installer',
    'HKCU:\\Software\\Policies\\Microsoft\\Windows\\Installer'
)) {{
    if (Test-Path $policyRoot) {{
        Set-ItemProperty -Path $policyRoot -Name 'DisableMSI' -Value 0 -ErrorAction SilentlyContinue
    }}
}}
sc.exe config msiserver start= demand | Out-Null
Start-Service -Name 'msiserver' -ErrorAction SilentlyContinue | Out-Null
Start-Process -FilePath 'msiexec.exe' -ArgumentList '/unregister' -Wait -ErrorAction SilentlyContinue | Out-Null
Start-Process -FilePath 'msiexec.exe' -ArgumentList '/regserver' -Wait -ErrorAction SilentlyContinue | Out-Null
""",
        )
            result = run_powershell(host, script)
            if result.rc == 0:
                return
            raise MsiServiceUnavailable(
                "Windows Installer service is unavailable (MSI ExitCode=1601)."
        )
        if "MSI ExitCode=1603" in stdout:
            _cleanup_orphaned_xp2p_msi(host)
            result = run_powershell(host, script)
            if result.rc == 0:
                return
        raise RuntimeError(
            "Failed to install xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            f"{log_context}{log_tail}"
        )


def _cleanup_orphaned_xp2p_msi(host: Host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
$productNamePattern = '^xp2p'
$roots = @(
    'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*',
    'HKLM:\\Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*'
)
$items = Get-ItemProperty -Path $roots -ErrorAction SilentlyContinue | Where-Object {
    $_.DisplayName -and $_.DisplayName -match $productNamePattern
}
foreach ($item in $items) {
    $code = $item.PSChildName
    if ($code -and $code -match '^\\{[0-9A-Fa-f-]+\\}$') {
        $args = @('/x', $code, '/qn', '/norestart')
        $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList $args -PassThru
        $proc.WaitForExit(300000) | Out-Null
    }
    if ($item.QuietUninstallString) {
        $cmd = $item.QuietUninstallString
    } elseif ($item.UninstallString) {
        $cmd = $item.UninstallString
    } else {
        $cmd = $null
    }
    if ($cmd) {
        $cmd = $cmd -replace '/I', '/X'
        Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', $cmd) -Wait -ErrorAction SilentlyContinue | Out-Null
    }
    if ($item.PSPath) {
        Remove-Item -Path $item.PSPath -Recurse -Force -ErrorAction SilentlyContinue
    }
}
$installerRoots = @(
    'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Installer\\UserData\\S-1-5-18\\Products',
    'HKLM:\\SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Installer\\UserData\\S-1-5-18\\Products'
)
$productKeys = @()
foreach ($root in $installerRoots) {
    $children = Get-ChildItem -Path $root -ErrorAction SilentlyContinue
    foreach ($child in $children) {
        $propsPath = Join-Path $child.PSPath 'InstallProperties'
        $props = Get-ItemProperty -Path $propsPath -ErrorAction SilentlyContinue
        if (-not $props) {
            continue
        }
        $name = $props.DisplayName
        if (-not $name) {
            $name = $props.ProductName
        }
        if ($name -and $name -match $productNamePattern) {
            $productKeys += $child.PSChildName
            Remove-Item -Path $child.PSPath -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}
$classRoots = @(
    'HKLM:\\SOFTWARE\\Classes\\Installer\\Products',
    'HKLM:\\SOFTWARE\\WOW6432Node\\Classes\\Installer\\Products'
)
foreach ($root in $classRoots) {
    foreach ($key in $productKeys) {
        $target = Join-Path $root $key
        if (Test-Path $target) {
            Remove-Item -Path $target -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}
"""
    run_powershell(host, script)


def _read_msi_log_tail(host: Host, path: Path, lines: int = 200) -> str:
    target = ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
$content = Get-Content -Path $target -Tail {lines}
$content
exit 0
"""
    result = run_powershell(host, script, label="msi_log_tail")
    if result.rc != 0:
        return "\nMSI log tail: <missing>\n"
    tail = (result.stdout or "").strip()
    if not tail:
        return "\nMSI log tail: <empty>\n"
    return "\nMSI log tail:\n" + tail + "\n"


def _read_msi_failure_context(host: Host, path: Path) -> str:
    target = ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
$lines = Get-Content -Path $target
$failIndex = -1
for ($i = $lines.Count - 1; $i -ge 0; $i--) {{
    if ($lines[$i] -match 'Return value 3') {{
        $failIndex = $i
        break
    }}
}}
if ($failIndex -lt 0) {{
    exit 0
}}
$startIndex = [Math]::Max(0, $failIndex - 40)
$lines[$startIndex..$failIndex]
exit 0
"""
    result = run_powershell(host, script, label="msi_log_context")
    if result.rc != 0:
        return ""
    context = (result.stdout or "").strip()
    if not context:
        return ""
    return "\nMSI failure context:\n" + context + "\n"


def uninstall_xp2p_from_msi(host: Host, msi_path: str | Path, *, purge_files: bool = True) -> None:
    msi_str = ps_quote(str(msi_path))
    install_dir = ps_quote(str(get_program_files_install_dir(host)))
    script = f"""
$ErrorActionPreference = 'Stop'
$msi = {msi_str}
$waitSeconds = 110
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
    if (-not $process.WaitForExit($waitSeconds * 1000)) {{
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Write-Output "MSI ExitCode=124"
        exit 124
    }}
    Write-Output ("MSI ExitCode=" + $process.ExitCode)
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
    result = run_powershell(host, script, label="get_remote_file_size")
    if result.rc != 0:
        stdout = result.stdout or ""
        if "MSI ExitCode=1601" in stdout:
            remove_services(host, ["xp2p-client", "xp2p-server"])
            remove_paths(
                host,
                [
                    PROGRAM_FILES_INSTALL_DIR,
                    PROGRAM_FILES_X86_INSTALL_DIR,
                    PROGRAM_DATA_ROOT,
                ],
                )
            print("WARNING: MSI uninstall failed (1601); cleaned up xp2p artifacts manually.")
            return
        if not path_exists(host, XP2P_EXE) and not service_exists(host, "xp2p-client") and not service_exists(
            host, "xp2p-server"
        ):
            print("WARNING: MSI uninstall reported failure, but xp2p artifacts are already removed.")
            return
        raise RuntimeError(
            "Failed to uninstall xp2p via MSI.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    remove_services(host, ["xp2p-client", "xp2p-server"])


def ensure_program_files_install(host: Host, *, force_reinstall: bool = False) -> None:
    if not force_reinstall:
        detected = _detect_xp2p_exe(host)
        if detected is not None:
            _set_install_paths_from_exe(detected)
            return

    start = time.perf_counter()
    msi_path = ensure_msi_package(host)
    print(f"TIMING: ensure_msi_package: {time.perf_counter() - start:.2f}s")
    start = time.perf_counter()
    try:
        install_xp2p_from_msi(host, msi_path)
    except MsiServiceUnavailable:
        detected = _detect_xp2p_exe(host)
        if detected is not None:
            _set_install_paths_from_exe(detected)
            return
        _manual_install_from_msi_bin(host)
        detected = _detect_xp2p_exe(host)
        if detected is not None:
            _set_install_paths_from_exe(detected)
            return
        raise
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


def get_program_files_install_dir(host: Host) -> Path:
    detected = _detect_xp2p_exe(host)
    if detected is not None:
        return _set_install_paths_from_exe(detected)
    return PROGRAM_FILES_INSTALL_DIR


def _manual_install_from_msi_bin(host: Host) -> None:
    install_dir = PROGRAM_FILES_INSTALL_DIR
    src_root = Path(r"C:\xp2p\build\msi-bin")
    script = f"""
$ErrorActionPreference = 'Stop'
$src = {ps_quote(str(src_root))}
$dst = {ps_quote(str(install_dir))}
$xp2p = Join-Path $src 'xp2p.exe'
$bundle = Join-Path $src 'bundle'
$xray = Join-Path $bundle 'xray.exe'
$wintun = Join-Path $bundle 'wintun.dll'
if (-not (Test-Path $xp2p)) {{
    throw "Fallback install failed: $xp2p not found"
}}
if (-not (Test-Path $xray)) {{
    throw "Fallback install failed: $xray not found"
}}
if (-not (Test-Path $wintun)) {{
    throw "Fallback install failed: $wintun not found"
}}
if (-not (Test-Path $dst)) {{
    New-Item -ItemType Directory -Path $dst -Force | Out-Null
}}
$bin = Join-Path $dst 'bin'
if (-not (Test-Path $bin)) {{
    New-Item -ItemType Directory -Path $bin -Force | Out-Null
}}
Copy-Item -Path $xp2p -Destination (Join-Path $dst 'xp2p.exe') -Force
Copy-Item -Path $xray -Destination (Join-Path $bin 'xray.exe') -Force
Copy-Item -Path $wintun -Destination (Join-Path $bin 'wintun.dll') -Force
"""
    run_powershell(host, script)


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
    result = run_powershell(host, script, label="detect_xp2p_exe_scan")
    if result.rc != 0:
        return _search_user_programs(host)
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return _search_user_programs(host)
    return Path(value[-1].strip())


def find_xp2p_exe(host: Host, hint_path: Path | None = None) -> Path | None:
    result = run_guest_script(
        host,
        "scripts/find_xp2p_exe.ps1",
        HintPath=str(hint_path) if hint_path else "",
    )
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
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
    result = run_powershell(host, script, label="search_user_programs")
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
    result = run_powershell(host, script, label="query_install_location")
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    return Path(value[-1].strip())


def _as_path(path: Path | str) -> Path:
    if isinstance(path, Path):
        return path
    return Path(str(path))


def _pending_candidate(path: Path) -> Path:
    if path.is_relative_to(CONFIG_PENDING_ROOT):
        return path
    if path.is_relative_to(CLIENT_CONFIG_DIR):
        return CLIENT_PENDING_DIR / path.relative_to(CLIENT_CONFIG_DIR)
    if path.is_relative_to(SERVER_CONFIG_DIR):
        return SERVER_PENDING_DIR / path.relative_to(SERVER_CONFIG_DIR)
    if path.is_relative_to(CONFIG_ROOT):
        return CONFIG_PENDING_ROOT / path.relative_to(CONFIG_ROOT)
    return path


def _resolve_config_path(host: Host, path: Path) -> Path:
    pending = _pending_candidate(path)
    if pending != path and _path_exists_raw(host, pending):
        return pending
    return path


def resolve_config_path(host: Host, path: Path | str) -> Path:
    return _resolve_config_path(host, _as_path(path))


def pending_candidate(path: Path | str) -> Path:
    return _pending_candidate(_as_path(path))


def ensure_admin_token(host: Host) -> None:
    last_error: Exception | None = None
    for attempt in range(1, 4):
        marker_local, marker_guest = _admin_token_marker()
        if marker_local.exists():
            marker_local.unlink(missing_ok=True)
        try:
            result = run_guest_script(
                host,
                "scripts/ensure_admin_token.ps1",
                MarkerPath=str(marker_guest),
                )
            if result.rc != 0:
                raise RuntimeError(
                    "Failed to ensure admin token.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
            if not marker_local.exists():
                probe = run_powershell(
                    host,
                    f"if (Test-Path {ps_quote(str(marker_guest))}) {{ exit 0 }} else {{ exit 3 }}",
                    )
                if probe.rc != 0:
                    raise RuntimeError("Admin token marker was not created on the guest.")
                marker_local.write_text("OK", encoding="ascii")
            return
        except pytest.skip.Exception as exc:
            last_error = exc
            backend = getattr(host, "backend", None)
            if backend is not None and hasattr(backend, "_reset_client"):
                backend._reset_client()
            if attempt < 3:
                print(f"WARNING: ensure_admin_token retry {attempt} after SSH error: {exc}")
                time.sleep(5)
                continue
            raise
        finally:
            if marker_local.exists():
                marker_local.unlink(missing_ok=True)
            run_powershell(
                host,
                f"if (Test-Path {ps_quote(str(marker_guest))}) {{ Remove-Item -Force {ps_quote(str(marker_guest))} }}",
                )
    if last_error is not None:
        raise last_error


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
    local_marker, guest_start, guest_done = _msi_build_markers()
    if local_marker.exists():
        local_marker.unlink(missing_ok=True)
    result = run_guest_script(
        host,
        "scripts/build_msi_package.ps1",
        Architecture=architecture,
        CacheDir=str(cache_dir),
        WixSource=wix_source,
        BuildId=_MSI_BUILD_ID or "",
        Marker=MSI_MARKER,
        StartMarkerPath=str(guest_start),
        DoneMarkerPath=str(guest_done),
        timeout=120,
    )
    if result.rc != 0:
        start_probe = run_powershell(
            host,
            f"if (Test-Path {ps_quote(str(guest_start))}) {{ exit 0 }} else {{ exit 3 }}",
            )
        done_probe = run_powershell(
            host,
            f"if (Test-Path {ps_quote(str(guest_done))}) {{ exit 0 }} else {{ exit 3 }}",
            )
        local_marker.write_text(
            f"start={start_probe.rc == 0} done={done_probe.rc == 0}",
            encoding="ascii",
            )
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
    resolved = _resolve_config_path(host, _as_path(path))
    if resolved != _as_path(path):
        return True
    return _path_exists_raw(host, path)


def _path_exists_guest(host: Host, path: Path | str) -> bool:
    result = run_guest_script(
        host,
        "scripts/path_exists.ps1",
        TargetPath=str(path),
    )
    return result.rc == 0


def _path_exists_raw(host: Host, path: Path | str) -> bool:
    target = ps_quote(str(path))
    result = run_powershell(
        host,
        f"if (Test-Path {target}) {{ exit 0 }} else {{ exit 3 }}",
    )
    return result.rc == 0


def remove_path(host: Host, path: Path | str) -> None:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    targets = [pending]
    if pending != resolved:
        targets.append(resolved)
    remove_paths(host, targets)


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
    script = """
$ErrorActionPreference = 'Stop'
$addresses = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin (@('Dhcp', 'Manual')) |
    Where-Object { $_.IPAddress -ne '127.0.0.1' } |
    Select-Object -ExpandProperty IPAddress
if (-not $addresses) {
    exit 3
}
$addresses
"""
    result = run_powershell(host, script, label="read_text")
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


def get_default_ipv4_sendthrough(host: Host) -> str | None:
    result = run_guest_script(
        host,
        "scripts/get_default_ipv4_sendthrough.ps1",
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to detect default IPv4 route address.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    values = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not values:
        return None
    return values[-1]


def get_interface_index(host: Host, interface_name: str) -> int:
    result = run_guest_script(
        host,
        "scripts/get_net_adapter_index.ps1",
        InterfaceName=interface_name,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query interface index.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    value = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not value:
        raise RuntimeError(f"No interface index returned for {interface_name!r}")
    try:
        return int(value[-1])
    except ValueError as exc:
        raise RuntimeError(f"Unexpected interface index output: {result.stdout!r}") from exc


def get_net_routes(
    host: Host,
    destination_prefix: str,
    interface_index: int | None = None,
) -> list[dict]:
    parameters: dict[str, object] = {
        "DestinationPrefix": destination_prefix,
    }
    if interface_index is not None:
        parameters["InterfaceIndex"] = str(interface_index)
    result = run_guest_script(
        host,
        "scripts/get_net_routes.ps1",
        **parameters,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query net routes.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    payload = (result.stdout or "").strip()
    if not payload:
        return []
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Unexpected net routes output: {payload!r}") from exc
    if data is None:
        return []
    if isinstance(data, dict):
        return [data]
    if isinstance(data, list):
        return data
    raise RuntimeError(f"Unexpected net routes output type: {type(data).__name__}")




def remove_tun_adapters(host: Host, adapter_names: Iterable[str]) -> None:
    names = [str(name) for name in adapter_names if str(name).strip()]
    if not names:
        return
    payload = base64.b64encode(json.dumps(names).encode("utf-8")).decode("ascii")
    result = run_guest_script(
        host,
        "scripts/remove_tun_adapters.ps1",
        NamesBase64=payload,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove TUN adapters.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def remove_paths(host: Host, paths: Iterable[Path | str]) -> None:
    targets = [str(path) for path in paths]
    if not targets:
        return
    target_list = ", ".join(ps_quote(path) for path in targets)
    script = f"""
$ErrorActionPreference = 'Stop'
$targets = @({target_list})
foreach ($target in $targets) {{
    if (-not $target) {{
        continue
    }}
    if (Test-Path $target) {{
        Remove-Item -Path $target -Force -Recurse -ErrorAction SilentlyContinue
    }}
}}
exit 0
"""
    result = run_powershell(host, script, label="write_text")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def stop_xp2p_processes(host: Host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
exit 0
"""
    result = run_powershell(host, script, label="remove_paths")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to stop xp2p processes.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def service_exists(host: Host, service_name: str) -> bool:
    result = run_guest_script(
        host,
        "scripts/check_service_exists.ps1",
        ServiceName=service_name,
    )
    return result.rc == 0


def remove_services(host: Host, services: Iterable[str]) -> None:
    payload = json.dumps([str(service) for service in services])
    if not payload:
        return
    encoded = base64.b64encode(payload.encode("utf-8")).decode("ascii")
    script = f"""
$ErrorActionPreference = 'Stop'
$payload = {ps_quote(encoded)}
$services = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($payload)) | ConvertFrom-Json
foreach ($name in $services) {{
    if (-not $name) {{
        continue
    }}
    Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    & sc.exe delete $name | Out-Null
}}
exit 0
"""
    result = run_powershell(host, script, label="stop_xp2p_processes")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove services on remote host.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def cleanup_xp2p_install(
    host: Host,
    *,
    config_dirs: Iterable[Path],
    state_files: Iterable[Path],
    extra_paths: Iterable[Path] = (),
) -> None:
    targets: list[Path | str] = []
    extra_state = [
        CONFIG_ROOT / ".apply",
        CONFIG_ROOT / APPLY_DIR_NAME,
        CONFIG_ROOT / APPLY_DIR_NAME / "apply.request",
        CONFIG_ROOT / APPLY_DIR_NAME / "apply.error",
        CONFIG_ROOT / "xp2p-client.toml",
        CONFIG_ROOT / "xp2p-server.toml",
        CONFIG_ROOT / "xp2p-client.toml.lkg",
        CONFIG_ROOT / "xp2p-server.toml.lkg",
        CONFIG_ROOT / "xp2p-client.state.json",
        CONFIG_ROOT / "xp2p-server.state.json",
        CONFIG_ROOT / "xp2p-client.state.json.lkg",
        CONFIG_ROOT / "xp2p-server.state.json.lkg",
        CONFIG_ROOT / "xp2p-client.tun-full.json",
        CONFIG_ROOT / "xp2p-server.tun-full.json",
        CONFIG_ROOT / "state-heartbeat.json",
        CONFIG_ROOT / "state-heartbeat-client.json",
        CONFIG_ROOT / "state-heartbeat-server.json",
        CONFIG_PENDING_ROOT,
        CONFIG_LIVE_ROOT,
        CONFIG_LKG_ROOT,
        CLIENT_PENDING_DIR,
        SERVER_PENDING_DIR,
        CLIENT_LIVE_DIR,
        SERVER_LIVE_DIR,
        CLIENT_CONFIG_DIR,
        SERVER_CONFIG_DIR,
        CLIENT_CONFIG_DIR / "inbounds.json.lkg",
        CLIENT_CONFIG_DIR / "outbounds.json.lkg",
        CLIENT_CONFIG_DIR / "routing.json.lkg",
        CLIENT_CONFIG_DIR / "logs.json.lkg",
        SERVER_CONFIG_DIR / "inbounds.json.lkg",
        SERVER_CONFIG_DIR / "outbounds.json.lkg",
        SERVER_CONFIG_DIR / "routing.json.lkg",
        SERVER_CONFIG_DIR / "logs.json.lkg",
        LOGS_DIR,
    ]
    for path in [*config_dirs, *state_files, *extra_paths, *extra_state]:
        resolved = _as_path(path)
        pending = _pending_candidate(resolved)
        targets.append(pending)
        if pending != resolved:
            targets.append(resolved)
    remove_paths(host, targets)


def cleanup_xp2p_leftovers(host: Host) -> None:
    remove_services(host, ["xp2p-client", "xp2p-server"])
    remove_paths(
        host,
        [
            PROGRAM_FILES_INSTALL_DIR,
            PROGRAM_FILES_X86_INSTALL_DIR,
            PROGRAM_DATA_ROOT,
        ],
    )


def _sanitize_dump_label(label: str) -> str:
    cleaned = []
    last_dash = False
    for char in (label or "").strip().lower():
        if char.isalnum() or char in {"-", "_"}:
            cleaned.append(char)
            last_dash = False
            continue
        if not last_dash:
            cleaned.append("-")
            last_dash = True
    value = "".join(cleaned).strip("-")
    return value or "failure"


def dump_failure_state(
    host: Host,
    *,
    label: str,
    extra_paths: Iterable[Path | str] = (),
) -> Path:
    timestamp = time.strftime("%Y%m%d-%H%M%S")
    backend = getattr(host, "backend", None)
    host_id = getattr(backend, "host", None) or getattr(backend, "hostname", None) or "host"
    safe_label = _sanitize_dump_label(label)
    dump_dir = GUEST_BUILD_ROOT / "logs" / "win"
    dump_path = dump_dir / f"xp2p-failure-{host_id}-{safe_label}-{timestamp}.log"
    ensure_dir = ps_quote(str(dump_dir))
    target = ps_quote(str(dump_path))
    run_powershell(
        host,
        f"""
$ErrorActionPreference = 'SilentlyContinue'
if (-not (Test-Path {ensure_dir})) {{
    New-Item -ItemType Directory -Path {ensure_dir} -Force | Out-Null
}}
if (Test-Path {target}) {{
    Remove-Item -Force {target} -ErrorAction SilentlyContinue
}}
'=== XP2P FAILURE DUMP ({safe_label}) {timestamp} ===' | Out-File -FilePath {target} -Encoding ASCII
""",
        label="dump_failure_state_init",
    )
    run_guest_script(
        host,
        "scripts/dump_net_state.ps1",
        OutputPath=str(dump_path),
        Label=label,
    )
    files = [
        CONFIG_ROOT / "xp2p-client.toml",
        CONFIG_ROOT / "xp2p-server.toml",
        CONFIG_ROOT / "xp2p-client.state.json",
        CONFIG_ROOT / "xp2p-server.state.json",
        CONFIG_ROOT / "state-heartbeat.json",
        CONFIG_ROOT / "state-heartbeat-client.json",
        CONFIG_ROOT / APPLY_DIR_NAME / "apply.request",
        CONFIG_PENDING_ROOT / "xp2p-client.toml",
        CONFIG_PENDING_ROOT / "xp2p-server.toml",
        CONFIG_LIVE_ROOT / "xp2p-client.toml",
        CONFIG_LIVE_ROOT / "xp2p-server.toml",
        CONFIG_LKG_ROOT / "xp2p-client.toml",
        CONFIG_LKG_ROOT / "xp2p-server.toml",
        CONFIG_ROOT / "config-client" / "inbounds.json",
        CONFIG_ROOT / "config-client" / "outbounds.json",
        CONFIG_ROOT / "config-client" / "routing.json",
        CONFIG_ROOT / "config-client" / "logs.json",
        CONFIG_ROOT / "config-server" / "inbounds.json",
        CONFIG_ROOT / "config-server" / "outbounds.json",
        CONFIG_ROOT / "config-server" / "routing.json",
        CONFIG_ROOT / "config-server" / "logs.json",
        CONFIG_PENDING_ROOT / "config-client" / "inbounds.json",
        CONFIG_PENDING_ROOT / "config-client" / "outbounds.json",
        CONFIG_PENDING_ROOT / "config-client" / "routing.json",
        CONFIG_PENDING_ROOT / "config-client" / "logs.json",
        CONFIG_PENDING_ROOT / "config-server" / "inbounds.json",
        CONFIG_PENDING_ROOT / "config-server" / "outbounds.json",
        CONFIG_PENDING_ROOT / "config-server" / "routing.json",
        CONFIG_PENDING_ROOT / "config-server" / "logs.json",
        CONFIG_LIVE_ROOT / "config-client" / "inbounds.json",
        CONFIG_LIVE_ROOT / "config-client" / "outbounds.json",
        CONFIG_LIVE_ROOT / "config-client" / "routing.json",
        CONFIG_LIVE_ROOT / "config-client" / "logs.json",
        CONFIG_LIVE_ROOT / "config-server" / "inbounds.json",
        CONFIG_LIVE_ROOT / "config-server" / "outbounds.json",
        CONFIG_LIVE_ROOT / "config-server" / "routing.json",
        CONFIG_LIVE_ROOT / "config-server" / "logs.json",
        CONFIG_LKG_ROOT / "config-client" / "inbounds.json",
        CONFIG_LKG_ROOT / "config-client" / "outbounds.json",
        CONFIG_LKG_ROOT / "config-client" / "routing.json",
        CONFIG_LKG_ROOT / "config-client" / "logs.json",
        CONFIG_LKG_ROOT / "config-server" / "inbounds.json",
        CONFIG_LKG_ROOT / "config-server" / "outbounds.json",
        CONFIG_LKG_ROOT / "config-server" / "routing.json",
        CONFIG_LKG_ROOT / "config-server" / "logs.json",
    ]
    files.extend(Path(path) for path in extra_paths)
    file_list = ", ".join(ps_quote(str(path)) for path in files)
    roots = ", ".join(
        ps_quote(str(path))
        for path in [
            CONFIG_ROOT,
            CONFIG_ROOT / APPLY_DIR_NAME,
            CONFIG_ROOT / ".apply",
            LOGS_DIR,
            CONFIG_PENDING_ROOT,
            CONFIG_LIVE_ROOT,
            CONFIG_LKG_ROOT,
        ]
    )
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$out = {target}
$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("")
$lines.Add("== config/log roots ==")
$roots = @({roots})
foreach ($root in $roots) {{
    if (-not $root) {{
        continue
    }}
    $lines.Add("-- $root --")
    if (Test-Path $root) {{
        Get-ChildItem -Path $root -Recurse -Force -ErrorAction SilentlyContinue |
            Select-Object FullName,Length,LastWriteTime |
            Format-Table -AutoSize | Out-String | ForEach-Object {{ $lines.Add($_) }}
    }} else {{
        $lines.Add("(missing)")
    }}
}}
$lines.Add("")
$lines.Add("== config/state files ==")
$paths = @({file_list})
foreach ($path in $paths) {{
    if (-not $path) {{
        continue
    }}
    if (Test-Path $path) {{
        $lines.Add("-- $path --")
        Get-Content -Path $path -Raw | ForEach-Object {{ $lines.Add($_) }}
    }}
}}
$lines.Add("")
$lines.Add("== log tails ==")
if (Test-Path {ps_quote(str(LOGS_DIR))}) {{
    $logs = Get-ChildItem -Path {ps_quote(str(LOGS_DIR))} -Filter *.log -Recurse -Force -ErrorAction SilentlyContinue
    foreach ($log in $logs) {{
        $lines.Add("-- $($log.FullName) (tail) --")
        Get-Content -Path $log.FullName -Tail 200 -ErrorAction SilentlyContinue | ForEach-Object {{ $lines.Add($_) }}
    }}
}}
$lines | Out-File -FilePath $out -Append -Encoding ASCII
"""
    run_powershell(host, script, label="dump_failure_state_files")
    print(f"Failure dump written: {dump_path}")
    return dump_path


def paths_exist(host: Host, paths: Iterable[Path | str]) -> set[str]:
    existing: set[str] = set()
    for path in paths:
        if path_exists(host, path):
            existing.add(str(path))
    return existing


def read_text(host: Host, path: Path | str) -> str:
    resolved = _resolve_config_path(host, _as_path(path))
    target = ps_quote(str(resolved))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
Get-Content -Path $target -Raw
exit 0
"""
    result = run_powershell(host, script, label="get_host_ipv4")
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
    resolved = _pending_candidate(_as_path(path))
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    target = ps_quote(str(resolved))
    payload = ps_quote(encoded)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$payload = {payload}
$bytes = [System.Convert]::FromBase64String($payload)
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$encoding = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($target, $text, $encoding)
exit 0
"""
    result = run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def write_apply_request(host: Host, role: str) -> None:
    timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    payload = json.dumps(
        {
            "id": str(uuid.uuid4()),
            "timestamp": timestamp,
            "role": role,
        }
    )
    payload = f"{payload}\n"
    path = CONFIG_ROOT / APPLY_DIR_NAME / "apply.request"
    write_text(host, path, payload)
