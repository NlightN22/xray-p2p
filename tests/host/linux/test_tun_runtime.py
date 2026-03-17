from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_ADDR = "198.18.0.1/30"
SERVER_ADDR = "198.18.0.5/30"


def _start_service(role: str, runner, host) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    result = runner(role, "service", "start", check=False)
    print(f"[runtime] xp2p {role} service start rc={result.rc}\n{result.stdout}\n{result.stderr}")
    if result.rc != 0:
        raise AssertionError(
            f"xp2p {role} service start failed (rc={result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _stop_service(role: str, runner) -> None:
    runner(role, "service", "stop")


def _dump_diagnostics(host, role: str) -> None:
    commands = [
        "date",
        "uname -a",
        f"systemctl status xp2p-{role}.service --no-pager || true",
        f"journalctl -u xp2p-{role}.service -n 200 --no-pager || true",
        f"ip -4 addr show dev {CLIENT_TUN} || true",
        f"ip -4 addr show dev {SERVER_TUN} || true",
        "ip rule show || true",
        "ip route show table 20090 || true",
        "ip route show table 20091 || true",
    ]
    for cmd in commands:
        result = host.run(f"sudo -n {cmd}")
        print(f"[diag] {cmd}\n{result.stdout}\n{result.stderr}")


def _assert_tun_addr(host, name: str, addr: str) -> None:
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/assert_tun_addr.sh",
        name,
        addr,
    )
    print(f"[runtime] tun addr check {name} {addr} rc={result.rc}\n{result.stdout}\n{result.stderr}")
    assert result.rc == 0, (
        "TUN address check failed.\n"
        f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_client_service_brings_up_tun(client_host, xp2p_client_runner):
    failed = False
    try:
        print("[runtime] client install start")
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.10.80",
            "--user",
            "tun-runtime-client@example.com",
            "--password",
            "tun-runtime-client-pass",
            "--force",
            check=True,
        )
        print("[runtime] client install done")
        _start_service("client", xp2p_client_runner, client_host)
        _assert_tun_addr(client_host, CLIENT_TUN, CLIENT_ADDR)
    except Exception:
        failed = True
        raise
    finally:
        if failed:
            _dump_diagnostics(client_host, "client")
        _stop_service("client", xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_service_brings_up_tun(server_host, xp2p_server_runner):
    failed = False
    try:
        print("[runtime] server install start")
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62140",
            "--host",
            "tun-runtime-server.example.com",
            "--force",
            check=True,
        )
        print("[runtime] server install done")
        _start_service("server", xp2p_server_runner, server_host)
        _assert_tun_addr(server_host, SERVER_TUN, SERVER_ADDR)
    except Exception:
        failed = True
        raise
    finally:
        if failed:
            _dump_diagnostics(server_host, "server")
        _stop_service("server", xp2p_server_runner)
