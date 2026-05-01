from __future__ import annotations

import base64
import json
import shlex
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from ._util import _posix


def path_exists(host: Host, path: str | Path | PurePosixPath) -> bool:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ -e \"$1\" ]; then exit 0; else exit 3; fi' "
        f"-- {quoted}"
    )
    if result.rc in (0, 3):
        return result.rc == 0
    raise RuntimeError(
        f"Failed to check path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def remove_path(host: Host, path: str | Path | PurePosixPath) -> None:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ -e \"$1\" ]; then rm -rf \"$1\"; exit 0; fi; exit 3' "
        f"-- {quoted}"
    )
    if result.rc not in (0, 3):
        raise RuntimeError(
            f"Failed to remove path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def read_text(host: Host, path: str | Path | PurePosixPath) -> str:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ ! -f \"$1\" ]; then exit 3; fi; cat \"$1\"' "
        f"-- {quoted}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout


def read_json(host: Host, path: str | Path | PurePosixPath) -> dict:
    content = read_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def write_text(host: Host, path: str | Path | PurePosixPath, content: str) -> None:
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    path_arg = _posix(path)
    quoted_path = shlex.quote(path_arg)
    quoted_content = shlex.quote(encoded)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ ! -d \"$(dirname \"$1\")\" ]; then exit 3; fi; "
        "printf %s \"$2\" | base64 -d >\"$1\"' "
        f"-- {quoted_path} {quoted_content}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def file_sha256(host: Host, path: str | Path | PurePosixPath) -> str:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ ! -f \"$1\" ]; then exit 3; fi; sha256sum \"$1\"' "
        f"-- {quoted}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to hash remote file {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return (result.stdout or "").split()[0]

