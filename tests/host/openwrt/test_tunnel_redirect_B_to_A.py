from __future__ import annotations

import re

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
DIAG_IP = "10.0.200.10"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_DOMAIN_IP = "10.0.200.11"
DIAG_DOMAIN_CIDR = f"{DIAG_DOMAIN_IP}/32"
DIAG_DOMAIN = "diag.service.internal"
HEARTBEAT_STATE_FILE = helpers.INSTALL_ROOT / "state-heartbeat.json"


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


def _find_interface_for_ip(host, ip: str) -> str:
    escaped = re.escape(ip)
    command = f"ip -o -4 addr show | awk '$4 ~ /^{escaped}\\// {{print $2; exit}}'"
    result = host.run(command)
    interface = (result.stdout or "").strip().splitlines()
    if not interface:
        pytest.fail(f"Unable to find interface for {ip} on {host.backend.hostname}. STDOUT: {result.stdout}")
    return interface[0]


def _add_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")
    add_result = host.run(f"ip addr add {cidr} dev {iface}")
    if add_result.rc != 0:
        pytest.fail(f"Failed to add IP alias {cidr} on {iface}: {add_result.stdout}\n{add_result.stderr}")


def _remove_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")


def _add_blackhole_route(host, cidr: str) -> None:
    host.run(f"ip route del {cidr} >/dev/null 2>&1 || true")
    result = host.run(f"ip route add blackhole {cidr}")
    if result.rc != 0:
        pytest.fail(f"Failed to add blackhole route {cidr}: {result.stdout}\n{result.stderr}")


def _remove_blackhole_route(host, cidr: str) -> None:
    host.run(f"ip route del {cidr} >/dev/null 2>&1 || true")


def _add_hosts_entry(host, ip: str, domain: str) -> None:
    pattern = re.escape(domain)
    host.run(f"sed -i '/\\s{pattern}$/d' /etc/hosts")
    append = host.run(f"printf '%s %s\\n' {ip} {domain} >> /etc/hosts")
    if append.rc != 0:
        pytest.fail(f"Failed to add hosts entry {domain}: {append.stdout}\n{append.stderr}")


def _remove_hosts_entry(host, domain: str) -> None:
    pattern = re.escape(domain)
    host.run(f"sed -i '/\\s{pattern}$/d' /etc/hosts")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_B_to_A(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    for machine, host in ((SERVER_MACHINE, server_host), (CLIENT_MACHINE, client_host)):
        openwrt_env.sync_build_output(machine)
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup(iface: str | None = None):
        for host in (server_host, client_host):
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        for host in (server_host, client_host):
            helpers.remove_path(host, HEARTBEAT_STATE_FILE)
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
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)

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

        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
        )
        helpers.assert_reverse_cli_output(
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        redirect_entries = []
        try:
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
            assert initial_ping.rc != 0
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
                "--tag",
                endpoint_tag,
                check=True,
            )
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
                "--tag",
                endpoint_tag,
                check=True,
            )

            for cidr, domain in ((DIAG_CIDR, None), (DIAG_DOMAIN, DIAG_DOMAIN)):
                args = [
                    "server",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--tag",
                    reverse_tag,
                ]
                if domain:
                    args.extend(["--domain", domain])
                    redirect_entries.append({"mode": "domain", "value": domain})
                else:
                    args.extend(["--cidr", cidr])
                    redirect_entries.append({"mode": "cidr", "value": cidr})
                server_runner(*args, check=True)

            with openwrt_env.xp2p_run_session(
                server_host,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
                helpers.SERVER_LOG_FILE,
            ), openwrt_env.xp2p_run_session(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
            ):
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

                redirected_domain = client_runner(
                    "ping",
                    DIAG_DOMAIN,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                assert "0% loss" in _combined_output(redirected_domain)

                domain_log = helpers.read_text(server_host, helpers.SERVER_LOG_FILE)
                assert domain_log.lower().count("ping received") > baseline_count
        finally:
            while redirect_entries:
                entry = redirect_entries.pop()
                args = [
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--tag",
                    reverse_tag,
                ]
                if entry["mode"] == "domain":
                    args.extend(["--domain", entry["value"]])
                else:
                    args.extend(["--cidr", entry["value"]])
                removal = server_runner(*args, check=False)
                stderr = _combined_output(removal)
                if removal.rc != 0 and "not found" not in stderr:
                    pytest.fail(
                        f"Failed to remove redirect {entry['value']}:\nSTDOUT:\n{removal.stdout}\nSTDERR:\n{removal.stderr}"
                    )
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
                "--tag",
                endpoint_tag,
                check=False,
            )
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
                "--tag",
                endpoint_tag,
                check=False,
            )
    finally:
        cleanup(iface_name)
