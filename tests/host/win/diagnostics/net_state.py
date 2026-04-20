from __future__ import annotations

from pathlib import Path

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host.win import env as win_env


def dump_net_state(host: Host, *, output_path: Path | str, label: str) -> CommandResult:
    return win_env.run_guest_script(
        host,
        "scripts/dump_net_state.ps1",
        OutputPath=str(output_path),
        Label=label,
    )
