import base64
import time

import pytest
from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

from ._sh_guest import (
    _guest_script_cache_key,
    _missing_script_error,
    _remote_sha256,
    _sha256_bytes,
    _stage_guest_script,
    run_guest_script,
)


def _refresh_ssh_host(host: Host, *, connect_timeout: int) -> Host | None:
    from . import env as _env

    machine = getattr(host, "_xp2p_machine", None)
    if not machine:
        return None
    try:
        refreshed = common.get_ssh_host(_env.VAGRANT_DIR, machine, connect_timeout=connect_timeout)
    except Exception:
        return None
    setattr(refreshed, "_xp2p_machine", machine)
    return refreshed


def encode_powershell(script: str) -> str:
    return base64.b64encode(script.encode("utf-16le")).decode("ascii")


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
    from . import env as _env

    encoded = encode_powershell(script)
    started = time.monotonic()
    effective_timeout = _env.DEFAULT_POWERSHELL_TIMEOUT if timeout is None else timeout
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

