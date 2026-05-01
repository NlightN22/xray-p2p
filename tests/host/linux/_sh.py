from __future__ import annotations

import shlex

from testinfra.backend.base import CommandResult
from testinfra.host import Host


def _run_shell(host: Host, script: str) -> CommandResult:
    quoted = shlex.quote(script)
    return host.run(f"bash -lc {quoted}")

