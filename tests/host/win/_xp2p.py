import base64
import json
import time
import uuid
from collections.abc import Iterable
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from ._xp2p_admin import ensure_admin_token
from ._xp2p_paths import _detect_xp2p_exe, _query_install_location, _search_user_programs, _set_install_paths_from_exe, find_xp2p_exe, get_program_files_install_dir


def _extract_marker(output: str, marker: str) -> str | None:
    for line in (output or "").splitlines():
        stripped = line.strip()
        if stripped.startswith(marker):
            return stripped[len(marker) :].strip()
    return None

def run_xp2p(
    host: Host,
    args: Iterable[str],
    *,
    timeout: int | float | None = None,
) -> CommandResult:
    from . import env as _env

    effective_timeout = _env.DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout

    def _run(attempt_host: Host) -> CommandResult:
        timeout_marker, guest_timeout_marker = _xp2p_timeout_marker()
        if timeout_marker.exists():
            timeout_marker.unlink(missing_ok=True)
        executor = ThreadPoolExecutor(max_workers=1)
        future = executor.submit(
            _run_xp2p_with_timeout_marker,
            attempt_host,
            args,
            effective_timeout,
            guest_timeout_marker,
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
        refreshed = _env._refresh_ssh_host(host, connect_timeout=30)
        if refreshed is None:
            raise
        print(
            "WARNING: xp2p command timed out without guest markers; retrying once with refreshed SSH connection."
        )
        return _run(refreshed)

def _run_xp2p_admin(
    host: Host,
    args: Iterable[str],
    *,
    timeout: int | float | None = None,
    timeout_marker: Path | None = None,
) -> CommandResult:
    from . import env as _env

    payload = _encode_args_payload(args)
    result = _env.run_guest_script(
        host,
        "scripts/run_xp2p_command.ps1",
        Xp2pPath=str(_env.XP2P_EXE),
        ArgsBase64=payload,
        TimeoutPath=str(timeout_marker) if timeout_marker else "",
        TimeoutSeconds=str(_env.DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout),
        timeout=_env.DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout,
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
    from . import env as _env

    arguments = [str(arg) for arg in args]
    arg_list = ", ".join(_env.ps_quote(arg) for arg in arguments)
    if not arg_list:
        arg_list = ""
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = {_env.ps_quote(str(_env.XP2P_EXE))}
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}
$arguments = @({arg_list})
& $xp2p @arguments
exit $LASTEXITCODE
"""
    return _env.run_powershell(
        host,
        script,
        timeout=_env.DEFAULT_XP2P_COMMAND_TIMEOUT if timeout is None else timeout,
    )

def _xp2p_requires_admin(args: Iterable[str]) -> bool:
    from . import env as _env

    parts = [str(arg).strip().lower() for arg in args if str(arg).strip()]
    if not parts:
        return False
    idx = 0
    while idx < len(parts) and parts[idx].startswith("-"):
        idx += 1
    if idx >= len(parts):
        return False
    if parts[idx] in {"client", "server"} and idx + 1 < len(parts):
        return parts[idx + 1] in _env.ADMIN_XP2P_SUBCOMMANDS
    return False

def _xp2p_timeout_marker() -> tuple[Path, Path]:
    from . import env as _env

    marker_dir = _env.REPO_ROOT / "build" / "xp2p-timeouts"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = uuid.uuid4().hex
    local_path = marker_dir / f"xp2p-timeout-{token}.txt"
    guest_path = _env.GUEST_BUILD_ROOT / "xp2p-timeouts" / f"xp2p-timeout-{token}.txt"
    return local_path, guest_path

def _admin_token_marker() -> tuple[Path, Path]:
    from . import env as _env

    marker_dir = _env.REPO_ROOT / "build" / "admin-token"
    marker_dir.mkdir(parents=True, exist_ok=True)
    token = uuid.uuid4().hex
    local_path = marker_dir / f"admin-token-{token}.txt"
    guest_path = Path(r"C:\Windows\Temp") / f"xp2p-admin-token-{token}.txt"
    return local_path, guest_path

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
