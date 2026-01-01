from __future__ import annotations

import re

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

DOMAIN = "srv.test.lan"
SERVER_DOMAIN = "srv-side.test.lan"
BASE_DOMAIN = "test.lan"
DNS_IP = "10.123.45.67"
SERVER_TUN_IP = "10.63.30.11"
CLIENT_TUN_IP = "10.63.30.12"
CORP_DOMAIN = "corp.test.com"
C1_FQDN = "c1.corp.test.com"
C1_DNS_IP = "10.0.101.132"
C1_DIAG_LISTEN = "0.0.0.0:62022"


@pytest.fixture(scope="module")
def xp2p_on_both(openwrt_server_host, openwrt_client_host, xp2p_openwrt_ipk):
    server_runner = lambda *args: openwrt_env.run_xp2p(openwrt_server_host, *args)
    client_runner = lambda *args: openwrt_env.run_xp2p(openwrt_client_host, *args)

    for host in (openwrt_server_host, openwrt_client_host):
        openwrt_env.cleanup_xp2p(host)
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk, force=True)

    server_ip = SERVER_TUN_IP
    server_install = server_runner(
        "server",
        "install",
        "--path",
        "/etc/xp2p",
        "--config-dir",
        "config-server",
        "--host",
        server_ip,
        "--force",
    )
    credential = helpers.extract_trojan_credential(server_install.stdout or "")
    client_runner(
        "client",
        "install",
        "--path",
        "/etc/xp2p",
        "--config-dir",
        "config-client",
        "--link",
        credential["link"],
        "--force",
    )

    yield xp2p_openwrt_ipk

    for host in (openwrt_server_host, openwrt_client_host):
        openwrt_env._stop_xp2p_services(host)
    helpers.cleanup_client_install(openwrt_client_host, client_runner)
    helpers.cleanup_server_install(openwrt_server_host, server_runner)
    for host in (openwrt_server_host, openwrt_client_host):
        openwrt_env.cleanup_xp2p(host)


def test_dns_forward_client_add_and_remove(openwrt_server_host, openwrt_client_host, xp2p_on_both):
    server_ip = SERVER_TUN_IP
    try:
        _ensure_dns_record(openwrt_server_host, DOMAIN, DNS_IP)
        _ensure_dns_record(openwrt_client_host, DOMAIN, DNS_IP)
        _reset_dnsforward_state(openwrt_client_host)

        add = openwrt_client_host.run(
            f"/usr/bin/xp2p client dns-forward add --domain {DOMAIN} --target {server_ip}:53 --with-forward --intercept --quiet"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"

        dhcp_show = openwrt_client_host.run("uci show dhcp")
        assert dhcp_show.rc == 0
        _assert_dns_server_entry(dhcp_show.stdout or "", DOMAIN, server_ip)
        _assert_rebind_allowed(dhcp_show.stdout or "", BASE_DOMAIN)

        fw_show = openwrt_client_host.run("uci show firewall | grep xp2p_dns_intercept || true")
        assert fw_show.rc == 0
        assert "xp2p_dns_intercept" in (fw_show.stdout or "")

        forward_port = _detect_forward_port(openwrt_client_host, server_ip)
        with openwrt_env.xp2p_run_session(
            openwrt_server_host,
            role="server",
            install_dir="/etc/xp2p",
            config_dir="config-server",
            log_path="/tmp/xp2p-server.log",
            ):
                with openwrt_env.xp2p_run_session(
                    openwrt_client_host,
                    role="client",
                    install_dir="/etc/xp2p",
                    config_dir="config-client",
                    log_path="/tmp/xp2p-client.log",
                ):
                    _wait_for_port(openwrt_client_host, forward_port)

                    _assert_dns_response(openwrt_client_host, DOMAIN, DNS_IP, server=f"127.0.0.1:{forward_port}")
                    _assert_dns_response(openwrt_client_host, DOMAIN, DNS_IP, server="127.0.0.1")

        remove = openwrt_client_host.run(
            f"/usr/bin/xp2p client dns-forward remove --domain {DOMAIN} --intercept --quiet"
        )
        assert remove.rc == 0, f"remove command failed: {remove.stderr}"

        dhcp_after = openwrt_client_host.run("uci show dhcp | grep xp2p_dns_ || true")
        assert dhcp_after.rc == 0
        assert DOMAIN not in (dhcp_after.stdout or "")

        rebind_after = openwrt_client_host.run("uci show dhcp | grep rebind_domain || true")
        assert rebind_after.rc == 0
        assert BASE_DOMAIN not in (rebind_after.stdout or "")

        fw_after = openwrt_client_host.run("uci show firewall | grep xp2p_dns_intercept || true")
        assert fw_after.rc == 0
        assert (fw_after.stdout or "").strip() == ""
    finally:
        _remove_dns_record(openwrt_server_host, DOMAIN, DNS_IP)
        _remove_dns_record(openwrt_client_host, DOMAIN, DNS_IP)
        _reset_dnsforward_state(openwrt_client_host)


def test_dns_forward_server_add_and_remove(openwrt_server_host, openwrt_client_host, xp2p_on_both):
    server_ip = SERVER_TUN_IP
    client_ip = CLIENT_TUN_IP
    try:
        _ensure_dns_record(openwrt_server_host, SERVER_DOMAIN, DNS_IP)
        _ensure_dns_record(openwrt_client_host, SERVER_DOMAIN, DNS_IP)
        _reset_dnsforward_state(openwrt_server_host)

        add = openwrt_server_host.run(
            f"/usr/bin/xp2p server dns-forward add --domain {SERVER_DOMAIN} --target {client_ip}:53 --with-forward --intercept --quiet"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"

        dhcp_show = openwrt_server_host.run("uci show dhcp")
        assert dhcp_show.rc == 0
        _assert_dns_server_entry(dhcp_show.stdout or "", SERVER_DOMAIN, client_ip, role="server")
        _assert_rebind_allowed(dhcp_show.stdout or "", BASE_DOMAIN)

        fw_show = openwrt_server_host.run("uci show firewall | grep xp2p_dns_intercept || true")
        assert fw_show.rc == 0
        assert "xp2p_dns_intercept" in (fw_show.stdout or "")

        forward_port = _detect_forward_port(openwrt_server_host, client_ip, role="server")
        with openwrt_env.xp2p_run_session(
            openwrt_server_host,
            role="server",
            install_dir="/etc/xp2p",
            config_dir="config-server",
            log_path="/tmp/xp2p-server.log",
        ):
            with openwrt_env.xp2p_run_session(
                openwrt_client_host,
                role="client",
                install_dir="/etc/xp2p",
                config_dir="config-client",
                log_path="/tmp/xp2p-client.log",
            ):
                _wait_for_port(openwrt_server_host, forward_port)

                lookup_direct = openwrt_server_host.run(f"nslookup {SERVER_DOMAIN} 127.0.0.1:{forward_port} || true")
                assert DNS_IP in (lookup_direct.stdout or ""), (
                    f"Expected {DNS_IP} via dokodemo port (rc={lookup_direct.rc}): {lookup_direct.stdout} {lookup_direct.stderr}"
                )

                lookup_intercept = openwrt_server_host.run(f"nslookup {SERVER_DOMAIN} 127.0.0.1 || true")
                assert DNS_IP in (lookup_intercept.stdout or ""), (
                    f"Expected {DNS_IP} via intercept (rc={lookup_intercept.rc}): {lookup_intercept.stdout} {lookup_intercept.stderr}"
                )

        remove = openwrt_server_host.run(
            f"/usr/bin/xp2p server dns-forward remove --domain {SERVER_DOMAIN} --intercept --quiet"
        )
        assert remove.rc == 0, f"remove command failed: {remove.stderr}"

        dhcp_after = openwrt_server_host.run("uci show dhcp | grep xp2p_dns_ || true")
        assert dhcp_after.rc == 0
        assert SERVER_DOMAIN not in (dhcp_after.stdout or "")

        rebind_after = openwrt_server_host.run("uci show dhcp | grep rebind_domain || true")
        assert rebind_after.rc == 0
        assert BASE_DOMAIN not in (rebind_after.stdout or "")

        fw_after = openwrt_server_host.run("uci show firewall | grep xp2p_dns_intercept || true")
        assert fw_after.rc == 0
        assert (fw_after.stdout or "").strip() == ""
    finally:
        _remove_dns_record(openwrt_server_host, SERVER_DOMAIN, DNS_IP)
        _remove_dns_record(openwrt_client_host, SERVER_DOMAIN, DNS_IP)
        _reset_dnsforward_state(openwrt_server_host)


def test_dns_forward_openwrt_b_with_c1_c2(
    openwrt_server_host,
    openwrt_client_host,
    alpine_c1_host,
    alpine_c2_host,
    xp2p_on_both,
):
    dns_forward_added = False
    diag_started = False
    dnsmasq_ready = False
    try:
        _alpine_guest(alpine_c1_host, "scripts/linux/setup_dnsmasq_alpine.sh", C1_FQDN)
        dnsmasq_ready = True
        _alpine_guest(alpine_c1_host, "scripts/linux/start_xp2p_diag.sh", C1_DIAG_LISTEN, "tcp")
        diag_started = True

        lookup_direct = openwrt_server_host.run(f"nslookup {C1_FQDN} {C1_DNS_IP} || true")
        assert C1_DNS_IP in (lookup_direct.stdout or ""), (
            f"Expected {C1_DNS_IP} from {C1_FQDN} via {C1_DNS_IP} "
            f"(rc={lookup_direct.rc}): {lookup_direct.stdout} {lookup_direct.stderr}"
        )

        _reset_dnsforward_state(openwrt_client_host)
        add = openwrt_client_host.run(
            f"/usr/bin/xp2p client dns-forward add --domain {CORP_DOMAIN} --target {C1_DNS_IP}:53 --intercept --with-forward --quiet"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"
        dns_forward_added = True

        forward_port = _detect_forward_port(openwrt_client_host, C1_DNS_IP)
        with openwrt_env.xp2p_run_session(
            openwrt_server_host,
            role="server",
            install_dir="/etc/xp2p",
            config_dir="config-server",
            log_path="/tmp/xp2p-server.log",
        ), openwrt_env.xp2p_run_session(
            openwrt_client_host,
            role="client",
            install_dir="/etc/xp2p",
            config_dir="config-client",
            log_path="/tmp/xp2p-client.log",
        ):
            _wait_for_port(openwrt_client_host, forward_port)
            _assert_dns_response(openwrt_client_host, C1_FQDN, C1_DNS_IP, server=f"127.0.0.1:{forward_port}")

            lookup_intercept = openwrt_client_host.run(f"nslookup {C1_FQDN} || true")
            assert C1_DNS_IP in (lookup_intercept.stdout or ""), (
                f"Expected {C1_DNS_IP} from {C1_FQDN} via intercept "
                f"(rc={lookup_intercept.rc}): {lookup_intercept.stdout} {lookup_intercept.stderr}"
            )

            c2_lookup = _alpine_guest(alpine_c2_host, "scripts/linux/nslookup.sh", C1_FQDN)
            assert C1_DNS_IP in (c2_lookup.stdout or ""), (
                f"Expected {C1_DNS_IP} from {C1_FQDN} on c2:\n{c2_lookup.stdout}\n{c2_lookup.stderr}"
            )

            c2_ping = _alpine_guest(
                alpine_c2_host, "scripts/linux/xp2p_ping.sh", C1_FQDN, "--count", "1"
            )
            tunnel_common.assert_zero_loss(c2_ping, f"to {C1_FQDN}")
    finally:
        if dns_forward_added:
            openwrt_client_host.run(
                f"/usr/bin/xp2p client dns-forward remove --domain {CORP_DOMAIN} --intercept --quiet"
            )
        _reset_dnsforward_state(openwrt_client_host)
        if diag_started:
            _alpine_guest(alpine_c1_host, "scripts/linux/stop_xp2p_diag.sh")
        if dnsmasq_ready:
            _alpine_guest(alpine_c1_host, "scripts/linux/cleanup_dnsmasq_alpine.sh")


def _alpine_guest(host, script: str, *args: str):
    result = openwrt_env.run_alpine_guest_script(host, script, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _ensure_dns_record(host, domain: str, ip: str) -> None:
    host.run(f"uci del_list dhcp.@dnsmasq[0].address='/{domain}/{ip}' >/dev/null 2>&1 || true")
    host.run(f"uci add_list dhcp.@dnsmasq[0].address='/{domain}/{ip}'")
    host.run("uci commit dhcp")
    host.run("/etc/init.d/dnsmasq reload")


def _remove_dns_record(host, domain: str, ip: str) -> None:
    host.run(f"uci del_list dhcp.@dnsmasq[0].address='/{domain}/{ip}' >/dev/null 2>&1 || true")
    host.run("uci commit dhcp")
    host.run("/etc/init.d/dnsmasq reload")


def _reset_dnsforward_state(host) -> None:
    host.run("for s in $(uci show dhcp 2>/dev/null | grep 'dhcp.xp2p_dns_' | cut -d. -f2 | cut -d= -f1); do uci delete dhcp.$s || true; done")
    host.run("uci -q delete dhcp.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci -q delete firewall.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci commit dhcp")
    host.run("uci commit firewall")
    host.run("/etc/init.d/dnsmasq reload")
    host.run("/etc/init.d/firewall reload")
    host.run("rm -f /etc/xp2p/dns-forward-state.json >/dev/null 2>&1 || true")


def _assert_dns_server_entry(output: str, domain: str, server_ip: str, role: str = "client") -> None:
    expected = f"/{domain}/127.0.0.1#"
    found = False
    for line in (output or "").splitlines():
        if ".server=" not in line:
            continue
        if expected in line:
            found = True
            break
    assert found, f"server entry for {domain} not found in dhcp config:\n{output}"


def _assert_rebind_allowed(output: str, base_domain: str) -> None:
    assert f"rebind_protection='0'" in output
    assert base_domain in output


def _detect_forward_port(host, target_ip: str, target_port: int = 53, role: str = "client") -> str:
    cmd = "/usr/bin/xp2p client forward list" if role == "client" else "/usr/bin/xp2p server forward list"
    list_res = host.run(cmd)
    assert list_res.rc == 0, f"forward list failed: {list_res.stderr}"
    output = list_res.stdout or ""
    # Expect a line like: "127.0.0.1:53331  tcp   <target_ip>:<port>"
    pattern = rf"127\.0\.0\.1:(\d+)\s+\S+\s+{re.escape(target_ip)}:{target_port}"
    match = re.search(pattern, output)
    assert match, f"could not detect forward port in output:\n{output}"
    return match.group(1)


def _assert_dns_response(host, domain: str, expected_ip: str, *, server: str) -> None:
    if ":" in server:
        host_part, port_part = server.rsplit(":", 1)
        dig_cmd = f"kdig +tcp @{host_part} -p {port_part} {domain} A +short"
    else:
        dig_cmd = f"kdig +tcp @{server} {domain} A +short"
    result = host.run(f"{dig_cmd} || true")
    stdout = result.stdout or ""
    if expected_ip in stdout:
        return
    logs = host.run(
        "echo '--- /tmp/xp2p-client.log ---'; "
        "cat /tmp/xp2p-client.log 2>/dev/null; "
        "echo '\n--- /tmp/xp2p-server.log ---'; "
        "cat /tmp/xp2p-server.log 2>/dev/null; "
        "echo '\n--- /tmp/xp2p-client-run.log ---'; "
        "cat /tmp/xp2p-client-run.log 2>/dev/null; "
        "echo '\n--- /tmp/xp2p-server-run.log ---'; "
        "cat /tmp/xp2p-server-run.log 2>/dev/null || true"
    )
    raise AssertionError(
        f"Expected {expected_ip} from {domain} via {server} (rc={result.rc}).\n"
        f"Command: {dig_cmd}\n"
        f"STDOUT:\n{stdout}\nSTDERR:\n{result.stderr}\n"
        f"XP2P logs:\n{logs.stdout}"
    )


def _wait_for_port(host, port: str, attempts: int = 15, delay: int = 1) -> None:
    check_cmd = (
        f"for i in $(seq 1 {attempts}); do "
        f"netstat -lnptu 2>/dev/null | grep -q ':{port} ' && exit 0; "
        f"sleep {delay}; "
        "done; exit 1"
    )
    res = host.run(check_cmd)
    assert res.rc == 0, f"port {port} did not open:\n{res.stdout}\n{res.stderr}"
