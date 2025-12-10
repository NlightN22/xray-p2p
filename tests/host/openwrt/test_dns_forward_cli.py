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
        _reset_client_state(openwrt_client_host)

        add = openwrt_client_host.run(
            f"/usr/bin/xp2p dns-forward add --domain {DOMAIN} --target {server_ip}:53 --intercept --quiet"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"

        dhcp_show = openwrt_client_host.run("uci show dhcp")
        assert dhcp_show.rc == 0
        _assert_dns_server_entry(dhcp_show.stdout or "", DOMAIN, server_ip)
        _assert_rebind_allowed(dhcp_show.stdout or "", BASE_DOMAIN)

        fw_show = openwrt_client_host.run("uci show firewall | grep xp2p_dns_intercept || true")
        assert fw_show.rc == 0
        assert "xp2p_dns_intercept" in (fw_show.stdout or "")

        lookup = openwrt_client_host.run(f"nslookup {DOMAIN} 127.0.0.1")
        assert lookup.rc == 0, f"nslookup failed: {lookup.stderr}"
        assert DNS_IP in (lookup.stdout or ""), f"Expected {DNS_IP} in nslookup output"

        remove = openwrt_client_host.run(
            f"/usr/bin/xp2p dns-forward remove --domain {DOMAIN} --intercept --quiet"
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
    host.run("for s in $(uci show dhcp 2>/dev/null | grep 'dhcp.xp2p_dns_' | cut -d. -f2 | cut -d= -f1); do uci delete dhcp.$s; done")
    host.run("uci -q delete dhcp.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci -q delete firewall.xp2p_dns_intercept >/dev/null 2>&1 || true")
    host.run("uci commit dhcp")
    host.run("uci commit firewall")
    host.run("/etc/init.d/dnsmasq reload")
    host.run("/etc/init.d/firewall reload")
    host.run("rm -f /etc/xp2p/dns-forward-state.json >/dev/null 2>&1 || true")


def _assert_dns_server_entry(output: str, domain: str, server_ip: str) -> None:
    pattern = rf"dhcp\\.xp2p_dns_.*\\.name=['\"]?{re.escape(domain)}['\"]?.*"
    assert re.search(pattern, output), f"server entry for {domain} not found:\n{output}"
    assert f"{server_ip}#53" in output
    assert "xp2p_dns_" in output


def _assert_rebind_allowed(output: str, base_domain: str) -> None:
    assert f"rebind_protection='0'" in output
    assert base_domain in output
