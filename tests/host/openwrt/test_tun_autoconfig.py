from __future__ import annotations

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_ADDR = "198.18.0.1/30"
SERVER_ADDR = "198.18.0.5/30"

pytestmark = [pytest.mark.host, pytest.mark.linux]


def _runner(host):
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
    helpers.cleanup_client_install(openwrt_host, runner)
    helpers.cleanup_server_install(openwrt_host, runner)
    helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
    return runner


def _uci_show_network(host, name: str) -> str:
    result = openwrt_env.run_guest_script(host, "scripts/openwrt/uci_network_show.sh", name)
    if result.rc != 0:
        pytest.fail(
            "uci show failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def _parse_uci_show(output: str) -> dict[str, list[str]]:
    values: dict[str, list[str]] = {}
    for raw in (output or "").splitlines():
        line = raw.strip()
        if not line or "=" not in line:
            continue
        key, val = line.split("=", 1)
        key = key.strip()
        value = val.strip().strip("'\"")
        values.setdefault(key, []).append(value)
    return values


def _assert_uci_interface(output: str, name: str, addr: str) -> None:
    data = _parse_uci_show(output)
    base = f"network.{name}"
    assert base in data and "interface" in data[base], f"Expected UCI interface section {base}"
    assert f"{base}.device" in data and name in data[f"{base}.device"]
    assert f"{base}.proto" in data and "static" in data[f"{base}.proto"]
    assert f"{base}.ipaddr" in data and addr in data[f"{base}.ipaddr"]
    assert f"{base}.xp2p_managed" in data and "1" in data[f"{base}.xp2p_managed"]


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_tun_autoconfig_client_network(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "openwrt-client@example.com",
            "--password",
            "openwrt-client-pass",
            "--force",
            check=True,
        )

        output = _uci_show_network(openwrt_host, CLIENT_TUN)
        _assert_uci_interface(output, CLIENT_TUN, CLIENT_ADDR)

        helpers.cleanup_client_install(openwrt_host, runner)
        assert not _uci_show_network(openwrt_host, CLIENT_TUN).strip(), (
            "Expected UCI interface to be removed after xp2p client remove"
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        openwrt_env.run_guest_script(openwrt_host, "scripts/openwrt/uci_network_delete.sh", CLIENT_TUN)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_tun_autoconfig_server_network(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62022",
            "--host",
            "openwrt-server.example",
            "--force",
            check=True,
        )

        output = _uci_show_network(openwrt_host, SERVER_TUN)
        _assert_uci_interface(output, SERVER_TUN, SERVER_ADDR)

        helpers.cleanup_server_install(openwrt_host, runner)
        assert not _uci_show_network(openwrt_host, SERVER_TUN).strip(), (
            "Expected UCI interface to be removed after xp2p server remove"
        )
    finally:
        helpers.cleanup_server_install(openwrt_host, runner)
        openwrt_env.run_guest_script(openwrt_host, "scripts/openwrt/uci_network_delete.sh", SERVER_TUN)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_tun_autoconfig_preserves_unmanaged_uci(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.21",
            "--user",
            "openwrt-manual@example.com",
            "--password",
            "openwrt-manual-pass",
            "--force",
            check=True,
        )

        openwrt_env.run_guest_script(
            openwrt_host,
            "scripts/openwrt/uci_network_set_manual.sh",
            CLIENT_TUN,
            CLIENT_TUN,
            "static",
            CLIENT_ADDR,
        )

        helpers.cleanup_client_install(openwrt_host, runner)

        output = _uci_show_network(openwrt_host, CLIENT_TUN)
        data = _parse_uci_show(output)
        base = f"network.{CLIENT_TUN}"
        assert base in data and "interface" in data[base], "Expected unmanaged UCI section to remain"
        assert f"{base}.xp2p_managed" not in data, "Expected unmanaged UCI section without xp2p_managed"
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        openwrt_env.run_guest_script(openwrt_host, "scripts/openwrt/uci_network_delete.sh", CLIENT_TUN)
