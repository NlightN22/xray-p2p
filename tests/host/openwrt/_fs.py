from __future__ import annotations

import shlex
from pathlib import Path, PurePosixPath

from testinfra.host import Host


def _posix(value: PurePosixPath | Path | str) -> str:
    if isinstance(value, (PurePosixPath, Path)):
        return value.as_posix()
    return str(value)


def _read_file_safe(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(path)
    result = host.run(f"cat {shlex.quote(target)} 2>/dev/null || true")
    if result.rc != 0:
        return ""
    return result.stdout or ""

