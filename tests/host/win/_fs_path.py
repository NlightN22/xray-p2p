from collections.abc import Iterable
from pathlib import Path

from testinfra.host import Host


def _as_path(path: Path | str) -> Path:
    if isinstance(path, Path):
        return path
    return Path(str(path))


def _pending_candidate(path: Path) -> Path:
    from . import env as _env

    if path.is_relative_to(_env.CONFIG_PENDING_ROOT):
        return path
    if path.is_relative_to(_env.CLIENT_CONFIG_DIR):
        return _env.CLIENT_PENDING_DIR / path.relative_to(_env.CLIENT_CONFIG_DIR)
    if path.is_relative_to(_env.SERVER_CONFIG_DIR):
        return _env.SERVER_PENDING_DIR / path.relative_to(_env.SERVER_CONFIG_DIR)
    if path.is_relative_to(_env.CONFIG_ROOT):
        return _env.CONFIG_PENDING_ROOT / path.relative_to(_env.CONFIG_ROOT)
    return path


def _resolve_config_path(host: Host, path: Path) -> Path:
    from . import env as _env

    pending = _pending_candidate(path)
    if pending != path and _env._path_exists_raw(host, pending):
        return pending
    return path


def resolve_config_path(host: Host, path: Path | str) -> Path:
    return _resolve_config_path(host, _as_path(path))


def pending_candidate(path: Path | str) -> Path:
    return _pending_candidate(_as_path(path))


def get_remote_file_size(host: Host, path: str | Path) -> int:
    from . import env as _env

    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
if (-not (Test-Path $target)) {{
    throw "File not found at $target"
}}
$item = Get-Item $target
Write-Output $item.Length
"""
    result = _env.run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to query remote file size.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        return int((result.stdout or "").strip().splitlines()[-1])
    except (ValueError, IndexError) as exc:
        raise RuntimeError(f"Unexpected size output: {result.stdout!r}") from exc


def path_exists(host: Host, path: Path | str) -> bool:
    from . import env as _env

    resolved = _resolve_config_path(host, _as_path(path))
    if resolved != _as_path(path):
        return True
    return _env._path_exists_raw(host, path)


def _path_exists_guest(host: Host, path: Path | str) -> bool:
    from . import env as _env

    result = _env.run_guest_script(
        host,
        "scripts/path_exists.ps1",
        TargetPath=str(path),
    )
    return result.rc == 0


def _path_exists_raw(host: Host, path: Path | str) -> bool:
    from . import env as _env

    target = _env.ps_quote(str(path))
    result = _env.run_powershell(
        host,
        f"if (Test-Path {target}) {{ exit 0 }} else {{ exit 3 }}",
        timeout=30,
        label="path_exists",
    )
    return result.rc == 0


def remove_path(host: Host, path: Path | str) -> None:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    targets = [pending]
    if pending != resolved:
        targets.append(resolved)
    remove_paths(host, targets)


def remove_paths(host: Host, paths: Iterable[Path | str]) -> None:
    from . import env as _env

    targets = [str(path) for path in paths]
    if not targets:
        return
    target_list = ", ".join(_env.ps_quote(path) for path in targets)
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
    result = _env.run_powershell(host, script, label="write_text")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def paths_exist(host: Host, paths: Iterable[Path | str]) -> set[str]:
    existing: set[str] = set()
    for path in paths:
        if path_exists(host, path):
            existing.add(str(path))
    return existing

