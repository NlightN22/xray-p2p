from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)
DIAG_IP = "10.77.0.1"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_DOMAIN_IP = "10.77.0.2"
DIAG_DOMAIN_CIDR = f"{DIAG_DOMAIN_IP}/32"
DIAG_DOMAIN = "diag.service.internal"


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = linux_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _find_interface_for_ip(host, ip: str) -> str:
    escaped = ip.replace(".", r"\.")
    command = f"ip -o -4 addr show | awk '$4 ~ /^{escaped}\\// {{print $2; exit}}'"
    result = host.run(command)
    interface = (result.stdout or "").strip().splitlines()
    if not interface:
        pytest.fail(f"Unable to find interface for {ip} on {host.backend.hostname}. STDOUT: {result.stdout}")
    return interface[0]


def _add_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"sudo -n ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")
    add_result = host.run(f"sudo -n ip addr add {cidr} dev {iface}")
    if add_result.rc != 0:
        pytest.fail(f"Failed to add IP alias {cidr} on {iface}: {add_result.stdout}\n{add_result.stderr}")


def _remove_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"sudo -n ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")


def _add_blackhole_route(host, cidr: str) -> None:
    host.run(f"sudo -n ip route del {cidr} >/dev/null 2>&1 || true")
    result = host.run(f"sudo -n ip route add blackhole {cidr}")
    if result.rc != 0:
        pytest.fail(f"Failed to add blackhole route {cidr}: {result.stdout}\n{result.stderr}")


def _remove_blackhole_route(host, cidr: str) -> None:
    host.run(f"sudo -n ip route del {cidr} >/dev/null 2>&1 || true")


def _add_hosts_entry(host, ip: str, domain: str) -> None:
    result = linux_env.run_guest_script(host, "scripts/linux/update_hosts_entry.sh", "add", ip, domain)
    if result.rc != 0:
        pytest.fail(
            "Failed to add hosts entry "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_hosts_entry(host, domain: str) -> None:
    linux_env.run_guest_script(host, "scripts/linux/update_hosts_entry.sh", "remove", domain)


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_B_to_A(linux_host_factory, xp2p_linux_versions):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)  # Host A
    client_host = linux_host_factory(linux_env.DEFAULT_SERVER)  # Host B
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup(iface: str | None = None):
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        for cidr in (DIAG_CIDR, DIAG_DOMAIN_CIDR):
            _remove_blackhole_route(client_host, cidr)
        _remove_hosts_entry(client_host, DIAG_DOMAIN)
        if iface:
            for alias in (DIAG_CIDR, DIAG_DOMAIN_CIDR):
                _remove_ip_alias(server_host, iface, alias)

    iface_name = _find_interface_for_ip(server_host, SERVER_IP)
    cleanup(iface_name)
    try:
        _add_ip_alias(server_host, iface_name, DIAG_CIDR)
        _add_ip_alias(server_host, iface_name, DIAG_DOMAIN_CIDR)
        _add_blackhole_route(client_host, DIAG_CIDR)
        _add_blackhole_route(client_host, DIAG_DOMAIN_CIDR)
        _add_hosts_entry(client_host, DIAG_DOMAIN_IP, DIAG_DOMAIN)

        server_install = server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"

        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            credential["link"],
            "--force",
            check=True,
        )

        with linux_env.xp2p_run_session(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ):
            with linux_env.xp2p_run_session(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
            ):
                baseline_log = helpers.read_text(server_host, helpers.SERVER_LOG_FILE)
                baseline_count = baseline_log.lower().count("ping received")

                initial_ping = client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=False,
                )
                initial_log = helpers.read_text(server_host, helpers.SERVER_LOG_FILE)
                assert initial_log.lower().count("ping received") == baseline_count
                initial_domain_ping = client_runner(
                    "ping",
                    DIAG_DOMAIN,
                    "--socks",
                    "--count",
                    "3",
                    check=False,
                )
                assert initial_domain_ping.rc != 0

                client_runner(
                    "client",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    "--cidr",
                    DIAG_CIDR,
                    "--host",
                    SERVER_IP,
                    check=True,
                )

                redirected_ping = client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                assert "0% loss" in _combined_output(redirected_ping)

                domain_before_rule = client_runner(
                    "ping",
                    DIAG_DOMAIN,
                    "--socks",
                    "--count",
                    "3",
                    check=False,
                )
                assert domain_before_rule.rc != 0

                redirect_list = client_runner(
                    "client",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert DIAG_CIDR in redirect_list

                routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
                helpers.assert_redirect_rule(routing, DIAG_CIDR, helpers.expected_proxy_tag(SERVER_IP))

                client_runner(
                    "client",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    "--domain",
                    DIAG_DOMAIN,
                    "--host",
                    SERVER_IP,
                    check=True,
                )

                redirect_list = client_runner(
                    "client",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert DIAG_DOMAIN in redirect_list

                routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
                helpers.assert_domain_redirect_rule(routing, DIAG_DOMAIN, helpers.expected_proxy_tag(SERVER_IP))

                client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    "--domain",
                    DIAG_DOMAIN,
                    "--host",
                    SERVER_IP,
                    check=True,
                )

                routing_after_domain_removal = helpers.read_json(
                    client_host, helpers.CLIENT_CONFIG_DIR / "routing.json"
                )
                helpers.assert_redirect_rule(routing_after_domain_removal, DIAG_CIDR, helpers.expected_proxy_tag(SERVER_IP))
                helpers.assert_no_domain_redirect_rule(
                    routing_after_domain_removal, DIAG_DOMAIN, helpers.expected_proxy_tag(SERVER_IP)
                )

                redirected_ping_after_domain = client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                assert "0% loss" in _combined_output(redirected_ping_after_domain)

                client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    "--cidr",
                    DIAG_CIDR,
                    check=True,
                )

                routing_after_remove = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
                helpers.assert_no_redirect_rule(routing_after_remove, DIAG_CIDR)
                helpers.assert_no_domain_redirect_rule(routing_after_remove, DIAG_DOMAIN)

                final_list = client_runner(
                    "client",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert "no redirect rules configured" in final_list.lower()
    finally:
        cleanup(iface_name)
