from __future__ import annotations

from pathlib import Path, PurePosixPath


def _posix(value: str | Path | PurePosixPath) -> str:
    if isinstance(value, (Path, PurePosixPath)):
        return value.as_posix()
    return str(value)


def _install_marker(marker: str, output: str | None) -> str | None:
    for line in (output or "").splitlines():
        line = line.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None

