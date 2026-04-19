from __future__ import annotations

import shlex
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
CLIENT_IP = "10.63.30.12"
CLIENT_REVERSE_TEST_IP = "10.0.102.50"
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
SERVER_FORWARD_PORT = 53341
CLIENT_FORWARD_PORT = 53331
CLIENT_REDIRECT_CIDR = "10.0.101.0/24"
SERVER_REDIRECT_CIDR = "10.0.102.0/24"
CLIENT_SOCKS_PORT = 51180
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE

REQUIRED_LIVE_ARTIFACTS = ("runtime.json", "xray.json")


def xray_configs_missing(host, config_dir) -> list[str]:
    if config_dir == helpers.CLIENT_CONFIG_DIR:
        live_dir = helpers.CLIENT_LIVE_DIR
    elif config_dir == helpers.SERVER_CONFIG_DIR:
        live_dir = helpers.SERVER_LIVE_DIR
    else:
        raise ValueError(f"Unsupported config dir: {config_dir}")
    return [
        (live_dir / name).as_posix()
        for name in REQUIRED_LIVE_ARTIFACTS
        if not helpers.path_exists_live(host, live_dir / name)
    ]


def wait_for_xray_configs(
    host,
    config_dir,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        missing = xray_configs_missing(host, config_dir)
        if not missing:
            return
        time.sleep(interval)
    missing = xray_configs_missing(host, config_dir)
    raise AssertionError(f"Missing xray configs (live): {missing}")


def wait_for_live_config(
    host,
    role: str,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    helpers.wait_for_live_config(
        host,
        role,
        timeout_seconds=timeout_seconds,
        poll_interval=interval,
    )


def wait_for_service_state(
    host,
    role: str,
    expected_active: bool,
    *,
    timeout_seconds: float = 45.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def is_xp2p_run_active(host, role: str) -> bool:
    cmd = (
        "ps w | "
        "grep -E "
        + shlex.quote(rf"xp2p {role} (run|service run)")
        + " | grep -v grep >/dev/null 2>&1"
    )
    return host.run(cmd).rc == 0


def ensure_service_running(host, role: str) -> None:
    if is_xp2p_run_active(host, role):
        return
    start = host.run(f"/etc/init.d/xp2p-{role} start")
    if start.rc != 0:
        pytest.fail(
            "Failed to start service "
            f"xp2p-{role} on {host.backend.hostname}.\nSTDOUT:\n{start.stdout}\nSTDERR:\n{start.stderr}"
        )
    wait_for_service_state(host, role, expected_active=True)


def current_mode(host, role: str) -> str:
    helpers.wait_for_live_config(host, role)
    if role == "client":
        config = helpers.read_live_client_config(host)
    elif role == "server":
        config = helpers.read_live_server_config(host)
    else:
        raise ValueError(f"Unsupported role: {role}")
    tun_enabled = config.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def set_mode(runner, role: str, config_dir: str, mode: str) -> None:
    runner(
        role,
        "mode",
        mode,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        config_dir,
        check=True,
    )


def ensure_mode(host, runner, role: str, config_dir: str, mode: str) -> str:
    current = current_mode(host, role)
    if current != mode:
        set_mode(runner, role, config_dir, mode)
    return current


def apply_pending_config(host, role: str, install_path: str, config_dir: str) -> None:
    helpers.state_pending_config(host, role)


def apply_pending_config_wait(host, role: str, install_path: str, config_dir: str) -> None:
    apply_pending_config(host, role, install_path, config_dir)


def wait_for_listen_port(host, port: int, *, timeout_seconds: float = 20.0, interval: float = 1.0) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        result = host.run(f"netstat -ltn 2>/dev/null | grep ':{port} '")
        if result.rc == 0:
            return
        time.sleep(interval)
    pytest.fail(f"Port {port} did not start listening on {host.backend.hostname} within {timeout_seconds}s")


def wait_for_ping_ready(
    runner,
    target: str,
    *,
    port: int | None = None,
    tunnel: bool = False,
    proto: str = "tcp",
    timeout_seconds: float = 30.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    last = None
    args = ["ping", target, "--count", "1", "--proto", proto]
    if port is not None:
        args.extend(["--port", str(port)])
    if tunnel:
        args.append("--tunnel")
    while time.time() < deadline:
        last = runner(*args, check=False)
        if last.rc == 0:
            return
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    pytest.fail(
        f"xp2p ping did not become ready for {target} within {timeout_seconds}s.\n"
        f"STDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )

