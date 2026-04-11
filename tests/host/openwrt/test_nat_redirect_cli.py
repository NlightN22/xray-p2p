from __future__ import annotations

import re

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env


def _current_mode(host) -> str:
    helpers.ensure_service_running(host, "client")
    helpers.wait_for_live_config(host, "client")
    state = helpers.read_live_client_config(host)
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in client config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _set_mode(host, mode: str) -> None:
    result = openwrt_env.run_xp2p(
        host,
        "client",
        "mode",
        mode,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
    )
    if result.rc != 0:
        raise AssertionError(
            f"Failed to set client mode to {mode}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.fixture(scope="module")
def xp2p_installed(openwrt_client_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_client_host, xp2p_openwrt_ipk, force=True)
    result = openwrt_env.run_xp2p(
        openwrt_client_host,
        "client",
        "install",
        "--path",
        "/etc/xp2p",
        "--config-dir",
        "config-client",
        "--host",
        "10.55.0.10",
        "--user",
        "nat-redirect@example.com",
        "--password",
        "nat-redirect-secret",
        "--force",
    )
    if result.rc != 0:
        raise RuntimeError(
            "xp2p client install failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    helpers.ensure_service_running(openwrt_client_host, "client")
    helpers.wait_for_live_config(openwrt_client_host, "client")
    return xp2p_openwrt_ipk


def test_nat_redirect_print_only_no_changes(openwrt_client_host, xp2p_installed):
    cidr = "10.123.0.0/24"
    port = "54321"
    snippet = "/tmp/xp2p-nat-test.nft"
    entry_dir = "/tmp/xp2p-nat-test.d"
    previous_mode = _current_mode(openwrt_client_host)
    if previous_mode != "proxy":
        _set_mode(openwrt_client_host, "proxy")
    try:
        add = openwrt_client_host.run(
            f"/usr/bin/xp2p nat-redirect add --cidr {cidr} --port {port} "
            f"--print-only --snippet {snippet} --entry-dir {entry_dir}"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"
        stdout = add.stdout or ""
        assert snippet in stdout
        assert entry_dir in stdout
        assert re.search(rf"iptables .* -d {re.escape(cidr)} .* --to-ports {port}", stdout)

        list_result = openwrt_client_host.run("/usr/bin/xp2p nat-redirect list")
        assert list_result.rc == 0
        assert "No transparent redirects configured." in (list_result.stdout or "")
    finally:
        if previous_mode != "proxy":
            _set_mode(openwrt_client_host, previous_mode)
