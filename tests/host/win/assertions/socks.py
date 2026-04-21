from __future__ import annotations

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env


def wait_for_tcp_listener(host: Host, *, port: int, timeout: int = 30) -> None:
    result = win_env.run_guest_script(
        host,
        "scripts/wait_for_tcp_listener.ps1",
        Port=str(port),
        TimeoutSeconds=str(timeout),
    )
    if result.rc != 0:
        pytest.fail(
            f"Timed out waiting for TCP listener on port {port}.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def wait_for_socks_listener(host: Host, *, port: int = 51180, timeout: int = 30) -> None:
    wait_for_tcp_listener(host, port=port, timeout=timeout)
