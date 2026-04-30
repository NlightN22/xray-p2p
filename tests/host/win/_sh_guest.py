import hashlib
import time
import uuid
from pathlib import Path

from testinfra.backend.base import CommandResult
from testinfra.host import Host


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
    from . import env as _env

    target = _env.ps_quote(str(path))
    script = (
        f"if (Test-Path {target}) {{ "
        f"(Get-FileHash -Algorithm SHA256 -Path {target}).Hash "
        "}"
    )
    result = _env.run_powershell(host, script, label="remote_sha256")
    if result.rc != 0:
        return None
    return (result.stdout or "").strip() or None


def _stage_guest_script(host: Host, relative: Path, *, relative_label: str) -> tuple[Path, Path]:
    from . import env as _env

    local_script = _env.LOCAL_GUEST_TESTS_ROOT / relative
    if not local_script.exists():
        raise RuntimeError(f"Guest script {relative_label} not found at {local_script}")
    staged_path = Path(r"C:\Windows\Temp") / f"xp2p-guest-script-{uuid.uuid4().hex}.ps1"
    _env.write_text(host, staged_path, local_script.read_text(encoding="utf-8"))
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
    from . import env as _env

    start = time.perf_counter()
    relative = Path(relative_path)
    script_path = _env.GUEST_TESTS_ROOT / relative
    cleanup_path: Path | None = None
    local_script = _env.LOCAL_GUEST_TESTS_ROOT / relative
    local_hash = _sha256_bytes(local_script.read_bytes())
    cache_key = _guest_script_cache_key(host, script_path)
    cached_hash = _env._GUEST_SCRIPT_CACHE.get(cache_key)
    use_cached = bool(cached_hash and cached_hash.lower() == local_hash.lower() and not force_stage)
    if not use_cached:
        if force_stage or not _env._path_exists_raw(host, script_path):
            script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
        else:
            remote_hash = _remote_sha256(host, script_path)
            if not remote_hash or remote_hash.lower() != local_hash.lower():
                script_path, cleanup_path = _stage_guest_script(host, relative, relative_label=relative_path)
            else:
                _env._GUEST_SCRIPT_CACHE[cache_key] = local_hash

    def _invoke(target: Path) -> CommandResult:
        ps_path = str(target).replace('"', '""')
        args = "".join(
            f" -{key} {_env._ps_quote(str(value))}" for key, value in parameters.items()
        )
        command = f"powershell -NoProfile -ExecutionPolicy Bypass -File \"{ps_path}\"{args}"
        effective_timeout = _env.DEFAULT_GUEST_SCRIPT_TIMEOUT if timeout is None else timeout
        print(f"Guest script start: {relative_path} (timeout={effective_timeout}s)")
        if parameters:
            print(f"Guest script params: {sorted(parameters.keys())}")
        return _env._ssh_run_with_refresh(
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
        if "result" in locals():
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
            _env.remove_path(host, cleanup_path)

