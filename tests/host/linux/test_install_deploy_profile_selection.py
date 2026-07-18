from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.serial]

HOST = "profile-selection.example.com"
PORT = "62330"
PROFILE = "vless-tls-vision"


def test_server_install_profile_reaches_live_runtime(server_host, xp2p_server_runner):
    try:
        xp2p_server_runner(
            "server",
            "service",
            "stop",
            check=False,
        )
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            HOST,
            "--port",
            PORT,
            "--profile",
            PROFILE,
            "--force",
            check=True,
        )

        desired = helpers.read_toml(server_host, helpers.SERVER_CONFIG_FILE).get("server") or {}
        assert desired.get("profile") == PROFILE

        xp2p_server_runner("server", "service", "start", check=True)
        _wait_for_path(server_host, helpers.SERVER_LIVE_DIR / "runtime.json")
        _wait_for_path(server_host, helpers.SERVER_LIVE_DIR / "xray.json")

        runtime = helpers.read_json(server_host, helpers.SERVER_LIVE_DIR / "runtime.json")
        subscription = ((runtime.get("control") or {}).get("subscription") or {})
        assert subscription.get("profile") == PROFILE
        assert subscription.get("protocol") == "vless"
        assert (subscription.get("parameters") or {}).get("flow") == "xtls-rprx-vision"

        xray = helpers.read_json(server_host, helpers.SERVER_LIVE_DIR / "xray.json")
        tunnel_inbound = _tunnel_inbound(xray)
        assert tunnel_inbound.get("protocol") == "vless"
    finally:
        xp2p_server_runner("server", "service", "stop", check=False)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)


def _wait_for_path(host, path, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if linux_env.path_exists(host, path):
            return
        time.sleep(1.0)
    raise AssertionError(f"Expected {path} to exist after {timeout} seconds")


def _tunnel_inbound(xray: dict) -> dict:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("tag") == "trojan-in":
            return inbound
    raise AssertionError(f"trojan-in inbound not found: {xray.get('inbounds')}")
