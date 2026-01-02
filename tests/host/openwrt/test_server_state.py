from __future__ import annotations

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

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


def _update_hosts_entry(host: Host, action: str, domain: str, ip: str | None = None) -> None:
    args = ["scripts/linux/update_hosts_entry_openwrt.sh", action]
    if action == "add":
        if not ip:
            pytest.fail("IP is required for add action")
        args.extend([ip, domain])
    else:
        args.append(domain)
    result = openwrt_env.run_guest_script(host, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_state_filters_non_server_entries(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.sync_build_output(openwrt_env.DEFAULT_OPENWRT_MACHINE)
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)

    runner = _runner(openwrt_host)
    server_domain = "srv-a.local"
    client_domain = "srv-b.local"
    trojan_port = "62070"
    client_user = "client-only@example.com"
    client_password = "client-only-pass"
    try:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.cleanup_server_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)

        primary_ip = helpers.detect_primary_ipv4(openwrt_host)
        _update_hosts_entry(openwrt_host, "add", server_domain, primary_ip)
        _update_hosts_entry(openwrt_host, "add", client_domain, primary_ip)

        server_install = runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            server_domain,
            "--port",
            trojan_port,
            "--force",
            check=True,
        )
        default_cred = helpers.extract_trojan_credential(server_install.stdout or "")

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            default_cred["link"],
            "--force",
            check=True,
        )

        fake_link = (
            f"trojan://{client_password}@{client_domain}:{trojan_port}?"
            f"allowInsecure=1&security=tls&sni={client_domain}#{client_user}"
        )
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            fake_link,
            check=True,
        )

        with openwrt_env.xp2p_run_session(
            openwrt_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ), openwrt_env.xp2p_run_session(
            openwrt_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE,
        ):
            heartbeat_state = helpers.wait_for_heartbeat_state(openwrt_host)
            helpers.assert_heartbeat_entry(
                heartbeat_state,
                helpers.expected_proxy_tag(server_domain),
                host=server_domain,
                user=default_cred["user"],
            )
            helpers.assert_heartbeat_entry(
                heartbeat_state,
                helpers.expected_proxy_tag(client_domain),
                host=client_domain,
                user=client_user,
            )

            server_state = runner(
                "server",
                "state",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                check=True,
            ).stdout or ""
            rows = tunnel_common.parse_state_rows(server_state)
            assert len(rows) == 1
            assert rows[0]["HOST"] == server_domain
            assert rows[0]["CLIENT_USER"] == default_cred["user"]
    finally:
        _update_hosts_entry(openwrt_host, "remove", server_domain)
        _update_hosts_entry(openwrt_host, "remove", client_domain)
        helpers.cleanup_server_install(openwrt_host, runner)
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
