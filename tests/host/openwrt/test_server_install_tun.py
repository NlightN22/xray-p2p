from __future__ import annotations

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]


def _runner(host: Host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _prepare_host(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk)
    runner = _runner(openwrt_host)
    helpers.cleanup_server_install(openwrt_host, runner)
    helpers.remove_path(openwrt_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
    return runner


def test_server_install_default_creates_tun_inbound(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            "tun-server.openwrt.test",
            "--port",
            "62022",
            "--force",
            check=True,
        )
        inbounds = helpers.read_preferred_json(openwrt_host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
        helpers.assert_tun_inbound(inbounds, "xp2ps")
    finally:
        helpers.cleanup_server_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.SERVER_HEARTBEAT_STATE_FILE)


def test_server_install_respects_tun_disabled(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        result = openwrt_env.run_xp2p_with_env(
            openwrt_host,
            {"XP2P_SERVER_TUN_ENABLED": "false"},
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            "tun-server-disabled.openwrt.test",
            "--port",
            "62023",
            "--force",
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        inbounds = helpers.read_preferred_json(openwrt_host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
        helpers.assert_no_tun_inbound(inbounds)
    finally:
        helpers.cleanup_server_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
