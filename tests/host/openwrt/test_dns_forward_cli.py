from __future__ import annotations

import re

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

DOMAIN = "srv.test.lan"
BASE_DOMAIN = "test.lan"
DNS_IP = "10.123.45.67"


@pytest.fixture(scope="module")
def xp2p_on_both(openwrt_server_host, openwrt_client_host, xp2p_openwrt_ipk):
    for host in (openwrt_server_host, openwrt_client_host):
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk, force=True)
    return xp2p_openwrt_ipk


def test_dns_forward_add_and_remove(openwrt_server_host, openwrt_client_host, xp2p_on_both):
    server_ip = helpers.detect_primary_ipv4(openwrt_server_host)
    try:
        _ensure_dns_record(openwrt_server_host, DOMAIN, DNS_IP)
        _ensure_dns_record(openwrt_client_host, DOMAIN, DNS_IP)
        _reset_client_state(openwrt_client_host)

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
        service_pid = _start_client_service(openwrt_client_host)
        _wait_for_port(openwrt_client_host, forward_port)

        lookup_direct = openwrt_client_host.run(f"nslookup {DOMAIN} 127.0.0.1:{forward_port} || true")
        assert DNS_IP in (lookup_direct.stdout or ""), (
            f"Expected {DNS_IP} via dokodemo port (rc={lookup_direct.rc}): {lookup_direct.stdout} {lookup_direct.stderr}"
        )

        lookup_intercept = openwrt_client_host.run(f"nslookup {DOMAIN} 127.0.0.1 || true")
        assert DNS_IP in (lookup_intercept.stdout or ""), (
            f"Expected {DNS_IP} via intercept (rc={lookup_intercept.rc}): {lookup_intercept.stdout} {lookup_intercept.stderr}"
        )

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
        _stop_client_service(openwrt_client_host, locals().get("service_pid"))
        _remove_dns_record(openwrt_server_host, DOMAIN, DNS_IP)
        _remove_dns_record(openwrt_client_host, DOMAIN, DNS_IP)
        _reset_client_state(openwrt_client_host)


def _ensure_dns_record(host, domain: str, ip: str) -> None:
    host.run(f"uci del_list dhcp.@dnsmasq[0].address='/{domain}/{ip}' >/dev/null 2>&1 || true")
    host.run(f"uci add_list dhcp.@dnsmasq[0].address='/{domain}/{ip}'")
    host.run("uci commit dhcp")
    host.run("/etc/init.d/dnsmasq reload")


def _remove_dns_record(host, domain: str, ip: str) -> None:
    host.run(f"uci del_list dhcp.@dnsmasq[0].address='/{domain}/{ip}' >/dev/null 2>&1 || true")
    host.run("uci commit dhcp")
    host.run("/etc/init.d/dnsmasq reload")


def _reset_client_state(host) -> None:
    host.run("for s in $(uci show dhcp 2>/dev/null | grep 'dhcp.xp2p_dns_' | cut -d. -f2 | cut -d= -f1); do uci delete dhcp.$s || true; done")
    host.run("uci -q delete dhcp.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci -q delete firewall.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci commit dhcp")
    host.run("uci commit firewall")
    host.run("/etc/init.d/dnsmasq reload")
    host.run("/etc/init.d/firewall reload")
    host.run("rm -f /etc/xp2p/dns-forward-state.json >/dev/null 2>&1 || true")


def _assert_dns_server_entry(output: str, domain: str, server_ip: str) -> None:
    marker = f"dhcp.xp2p_dns_{domain.replace('.', '_')}"
    assert f"{marker}.name='{domain}'" in output or f'{marker}.name="{domain}"' in output, (
        f"server entry for {domain} not found:\n{output}"
    )
    server_match = re.search(rf"{marker}\.server='([^']+#\d+)'", output)
    assert server_match, f"server value missing for {domain}:\n{output}"
    server_value = server_match.group(1)
    assert "#" in server_value, f"unexpected server value for {domain}: {server_value}\n{output}"
    assert "xp2p_dns_" in output, f"xp2p dns section missing:\n{output}"


def _assert_rebind_allowed(output: str, base_domain: str) -> None:
    assert f"rebind_protection='0'" in output
    assert base_domain in output


def _detect_forward_port(host, target_ip: str, target_port: int = 53) -> str:
    list_res = host.run("/usr/bin/xp2p client forward list")
    assert list_res.rc == 0, f"forward list failed: {list_res.stderr}"
    output = list_res.stdout or ""
    # Expect a line like: "127.0.0.1:53331  tcp   <target_ip>:<port>"
    pattern = rf"127\.0\.0\.1:(\d+)\s+\S+\s+{re.escape(target_ip)}:{target_port}"
    match = re.search(pattern, output)
    assert match, f"could not detect forward port in output:\n{output}"
    return match.group(1)


def _start_client_service(host):
    start = host.run(
        "nohup /usr/bin/xp2p client service run --heartbeat=false --max-restarts=0 >/tmp/xp2p-client.log 2>&1 & echo $!"
    )
    assert start.rc == 0, f"failed to start client service: {start.stderr}"
    pid = (start.stdout or "").strip()
    return pid


def _stop_client_service(host, pid):
    if not pid:
        return
    host.run(f"kill {pid} >/dev/null 2>&1 || true")
    host.run("sleep 1")


def _wait_for_port(host, port: str, attempts: int = 15, delay: int = 1) -> None:
    check_cmd = (
        f"for i in $(seq 1 {attempts}); do "
        f"netstat -lnptu 2>/dev/null | grep -q ':{port} ' && exit 0; "
        f"sleep {delay}; "
        "done; exit 1"
    )
    res = host.run(check_cmd)
    assert res.rc == 0, f"port {port} did not open:\n{res.stdout}\n{res.stderr}"
