from __future__ import annotations

import json
import re
import time

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
C1_DIAG_LISTEN = "0.0.0.0:62022"
C1_LAN_GATEWAY = "10.0.101.1"
C2_LAN_GATEWAY = "10.0.102.1"


def _current_mode(host, role: str) -> str:
    helpers.wait_for_live_config(host, role)
    if role == "client":
        config = helpers.read_live_client_config(host)
    elif role == "server":
        config = helpers.read_live_server_config(host)
    else:
        raise ValueError(f"Unsupported role: {role}")
    tun_enabled = config.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _set_mode(runner, role: str, config_dir: str, mode: str) -> None:
    result = runner(
        role,
        "mode",
        mode,
        "--path",
        "/etc/xp2p",
        "--config-dir",
        config_dir,
    )
    if result.rc != 0:
        raise AssertionError(
            "xp2p command failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _ensure_mode(host, runner, role: str, config_dir: str, mode: str) -> str:
    current = _current_mode(host, role)
    if current != mode:
        _set_mode(runner, role, config_dir, mode)
    return current


def _apply_pending_config(host, role: str) -> None:
    helpers.apply_pending_config(host, role)


def _wait_for_ping_ready(
    runner,
    target: str,
    *args: str,
    timeout_seconds: float = 30.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    last = None
    ping_args = ["ping", target, "--count", "1", *args]
    while time.time() < deadline:
        last = runner(*ping_args)
        if last.rc == 0:
            return
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    pytest.fail(
        f"xp2p ping did not become ready for {target} within {timeout_seconds}s.\n"
        f"STDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


@pytest.fixture(scope="module")
def xp2p_on_both(openwrt_server_host, openwrt_client_host, xp2p_openwrt_ipk):
    server_runner = lambda *args: openwrt_env.run_xp2p_live(openwrt_server_host, *args)
    client_runner = lambda *args: openwrt_env.run_xp2p_live(openwrt_client_host, *args)

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
    _apply_pending_config(openwrt_server_host, "server")
    _apply_pending_config(openwrt_client_host, "client")

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

        fw_show = openwrt_client_host.run("uci show firewall | grep Intercept-DNS || true")
        assert fw_show.rc == 0
        assert "Intercept-DNS" in (fw_show.stdout or "")

        _apply_pending_config(openwrt_client_host, "client")
        forward_port = _detect_forward_port(openwrt_client_host, server_ip)
        helpers.ensure_service_running(openwrt_server_host, "server")
        helpers.ensure_service_running(openwrt_client_host, "client")
        helpers.wait_for_apply_request_clear(openwrt_server_host, timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(openwrt_client_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_server_host, "server")
        helpers.wait_for_live_config(openwrt_client_host, "client")
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

        fw_after = openwrt_client_host.run("uci show firewall | grep Intercept-DNS || true")
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

        fw_show = openwrt_server_host.run("uci show firewall | grep Intercept-DNS || true")
        assert fw_show.rc == 0
        assert "Intercept-DNS" in (fw_show.stdout or "")

        _apply_pending_config(openwrt_server_host, "server")
        forward_port = _detect_forward_port(openwrt_server_host, client_ip, role="server")
        helpers.ensure_service_running(openwrt_server_host, "server")
        helpers.ensure_service_running(openwrt_client_host, "client")
        helpers.wait_for_apply_request_clear(openwrt_server_host, timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(openwrt_client_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_server_host, "server")
        helpers.wait_for_live_config(openwrt_client_host, "client")
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

        fw_after = openwrt_server_host.run("uci show firewall | grep Intercept-DNS || true")
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
    redirect_added = False
    nat_added = False
    previous_client_mode = None
    client_runner = lambda *args: openwrt_env.run_xp2p_live(openwrt_client_host, *args)
    endpoint_tag = helpers.expected_proxy_tag(SERVER_TUN_IP)
    try:
        c1_dns_ip = _detect_alpine_ipv4(alpine_c1_host)
        _alpine_guest(alpine_c1_host, "scripts/linux/ensure_route.sh", "10.0.102.0/24", C1_LAN_GATEWAY)
        _alpine_guest(alpine_c2_host, "scripts/linux/ensure_route.sh", "10.0.101.0/24", C2_LAN_GATEWAY)
        _alpine_guest(alpine_c1_host, "scripts/linux/setup_dnsmasq_alpine.sh", C1_FQDN)
        dnsmasq_ready = True
        _alpine_guest(alpine_c1_host, "scripts/linux/start_xp2p_diag.sh", C1_DIAG_LISTEN, "tcp")
        diag_started = True

        direct_ping = openwrt_env.run_xp2p_live(openwrt_server_host, "ping", c1_dns_ip, "--count", "1")
        if direct_ping.rc != 0:
            debug = _dump_dns_forward_debug(
                openwrt_client_host, openwrt_server_host, alpine_c1_host, alpine_c2_host, c1_dns_ip
            )
            raise AssertionError(
                "openwrt-a -> c1 ping failed.\n"
                f"STDOUT:\n{direct_ping.stdout}\nSTDERR:\n{direct_ping.stderr}\n\n"
                f"DNS forward debug:\n{debug}"
            )
        tunnel_common.assert_zero_loss(direct_ping, "openwrt-a to c1")

        lookup_direct = _openwrt_nslookup(openwrt_server_host, C1_FQDN, server=c1_dns_ip)
        assert c1_dns_ip in (lookup_direct.stdout or ""), (
            f"Expected {c1_dns_ip} from {C1_FQDN} via {c1_dns_ip} "
            f"(rc={lookup_direct.rc}): {lookup_direct.stdout} {lookup_direct.stderr}"
        )

        _reset_dnsforward_state(openwrt_client_host)
        add = openwrt_client_host.run(
            f"/usr/bin/xp2p client dns-forward add --domain {CORP_DOMAIN} --target {c1_dns_ip}:53 --intercept --with-forward --quiet"
        )
        assert add.rc == 0, f"add command failed: {add.stderr}"
        dns_forward_added = True

        helpers.ensure_service_running(openwrt_server_host, "server")
        helpers.ensure_service_running(openwrt_client_host, "client")
        helpers.wait_for_apply_request_clear(openwrt_server_host, timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(openwrt_client_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_server_host, "server")
        helpers.wait_for_live_config(openwrt_client_host, "client")
        _wait_for_port(openwrt_client_host, "51180")
        _ensure_mode(
            openwrt_client_host,
            client_runner,
            "client",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "tun",
        )
        _apply_pending_config(openwrt_client_host, "client")
        helpers.wait_for_heartbeat_state(
            openwrt_server_host,
            path=helpers.SERVER_HEARTBEAT_STATE_FILE,
        )
        helpers.wait_for_heartbeat_state(
            openwrt_client_host,
            path=helpers.CLIENT_HEARTBEAT_STATE_FILE,
        )
        time.sleep(1.5)
        _wait_for_ping_ready(client_runner, SERVER_TUN_IP, "--tunnel", timeout_seconds=45.0)
        tunnel_ping = client_runner("ping", SERVER_TUN_IP, "--tunnel", "--count", "1")
        if tunnel_ping.rc != 0:
            debug = _dump_dns_forward_debug(
                openwrt_client_host, openwrt_server_host, alpine_c1_host, alpine_c2_host, c1_dns_ip
            )
            raise AssertionError(
                "openwrt-b tunnel ping failed.\n"
                f"STDOUT:\n{tunnel_ping.stdout}\nSTDERR:\n{tunnel_ping.stderr}\n\n"
                f"DNS forward debug:\n{debug}"
            )
        tunnel_common.assert_zero_loss(tunnel_ping, f"tunnel to {SERVER_TUN_IP}")

        redirect = client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            "/etc/xp2p",
            "--config-dir",
            "config-client",
            "--cidr",
            "10.0.101.0/24",
            "--tag",
            endpoint_tag,
        )
        assert redirect.rc == 0, f"redirect add failed: {redirect.stderr}"
        redirect_added = True
        _apply_pending_config(openwrt_client_host, "client")
        routing = helpers.read_live_json(openwrt_client_host, "/etc/xp2p/config-client/routing.json")
        helpers.assert_redirect_rule(routing, "10.0.101.0/24", endpoint_tag)

        _apply_pending_config(openwrt_client_host, "client")
        forward_port = _detect_forward_port(openwrt_client_host, c1_dns_ip)
        previous_client_mode = _ensure_mode(
            openwrt_client_host,
            client_runner,
            "client",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "proxy",
        )
        _apply_pending_config(openwrt_client_host, "client")
        inbounds_path = helpers.CLIENT_CONFIG_DIR / "inbounds.json"
        if not helpers.path_exists_live(openwrt_client_host, inbounds_path):
            raise AssertionError(f"Missing live client inbounds at {inbounds_path}")
        nat_port = _detect_dokodemo_port(openwrt_client_host, inbounds_path.as_posix())
        nat = client_runner(
            "nat-redirect",
            "add",
            "--cidr",
            "10.0.101.0/24",
            "--port",
            str(nat_port),
            "--quiet",
        )
        assert nat.rc == 0, f"nat-redirect add failed: {nat.stderr}"
        nat_added = True
        _apply_pending_config(openwrt_client_host, "client")

        helpers.ensure_service_running(openwrt_server_host, "server")
        helpers.ensure_service_running(openwrt_client_host, "client")
        helpers.wait_for_apply_request_clear(openwrt_server_host, timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(openwrt_client_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_server_host, "server")
        helpers.wait_for_live_config(openwrt_client_host, "client")
        _wait_for_port(openwrt_client_host, forward_port)
        _assert_dns_response(
            openwrt_client_host, C1_FQDN, c1_dns_ip, server=f"127.0.0.1:{forward_port}"
        )

        lookup_intercept = _openwrt_nslookup(openwrt_client_host, C1_FQDN)
        assert c1_dns_ip in (lookup_intercept.stdout or ""), (
            f"Expected {c1_dns_ip} from {C1_FQDN} via intercept "
            f"(rc={lookup_intercept.rc}): {lookup_intercept.stdout} {lookup_intercept.stderr}"
        )

        c2_lookup = _alpine_guest(alpine_c2_host, "scripts/linux/nslookup.sh", C1_FQDN)
        assert c1_dns_ip in (c2_lookup.stdout or ""), (
            f"Expected {c1_dns_ip} from {C1_FQDN} on c2:\n{c2_lookup.stdout}\n{c2_lookup.stderr}"
        )

        c2_ping = openwrt_env.run_alpine_guest_script(
            alpine_c2_host, "scripts/linux/xp2p_ping.sh", C1_FQDN, "--count", "1"
        )
        if c2_ping.rc != 0:
            debug = _dump_dns_forward_debug(
                openwrt_client_host, openwrt_server_host, alpine_c1_host, alpine_c2_host, c1_dns_ip
            )
            raise AssertionError(
                "xp2p ping from c2 failed.\n"
                f"STDOUT:\n{c2_ping.stdout}\nSTDERR:\n{c2_ping.stderr}\n\n"
                f"DNS forward debug:\n{debug}"
            )
        tunnel_common.assert_zero_loss(c2_ping, f"to {C1_FQDN}")
    except AssertionError as exc:
        debug = _dump_dns_forward_debug(
            openwrt_client_host, openwrt_server_host, alpine_c1_host, alpine_c2_host, c1_dns_ip
        )
        raise AssertionError(f"{exc}\n\nDNS forward debug:\n{debug}") from exc
    finally:
        if dns_forward_added:
            openwrt_client_host.run(
                f"/usr/bin/xp2p client dns-forward remove --domain {CORP_DOMAIN} --intercept --quiet"
            )
        if nat_added:
            if previous_client_mode and previous_client_mode != "proxy":
                _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy")
            client_runner("nat-redirect", "remove", "--all")
            if previous_client_mode and previous_client_mode != "proxy":
                _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
        if redirect_added:
            client_runner(
                "client",
                "redirect",
                "remove",
                "--path",
                "/etc/xp2p",
                "--config-dir",
                "config-client",
                "--cidr",
                "10.0.101.0/24",
                "--tag",
                endpoint_tag,
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


def _openwrt_nslookup(host, name: str, *, server: str | None = None):
    timeout = "5"
    retry = "1"
    if server:
        cmd = f"nslookup -timeout={timeout} -retry={retry} {name} {server} || true"
    else:
        cmd = f"nslookup -timeout={timeout} -retry={retry} {name} || true"
    return host.run(cmd)


def _detect_dokodemo_port(host, path: str) -> int:
    raw = helpers.read_text(host, path)
    doc = json.loads(raw or "{}")
    for inbound in doc.get("inbounds") or []:
        if (inbound.get("protocol") or "").strip() != "dokodemo-door":
            continue
        settings = inbound.get("settings") or {}
        if not settings.get("followRedirect"):
            continue
        port = int(inbound.get("port") or 0)
        if port > 0:
            return port
    raise AssertionError(f"Expected dokodemo-door inbound with followRedirect in {path}")


def _dump_dns_forward_debug(
    openwrt_client_host,
    openwrt_server_host,
    alpine_c1_host,
    alpine_c2_host,
    c1_dns_ip: str,
) -> str:
    parts: list[str] = []
    parts.append("--- openwrt-b xp2p redirect list ---")
    parts.append((openwrt_env.run_xp2p_live(openwrt_client_host, "client", "redirect", "list").stdout or "").strip())
    parts.append("--- openwrt-b nat-redirect list ---")
    parts.append((openwrt_client_host.run("/usr/bin/xp2p nat-redirect list || true").stdout or "").strip())
    parts.append("--- openwrt-b xp2p forward list ---")
    parts.append((openwrt_env.run_xp2p_live(openwrt_client_host, "client", "forward", "list").stdout or "").strip())
    parts.append("--- openwrt-b nft xray_transparent ---")
    parts.append((openwrt_client_host.run("nft list table inet xray_transparent 2>/dev/null || true").stdout or "").strip())
    parts.append("--- openwrt-b nft fw4 ---")
    parts.append((openwrt_client_host.run("nft list table inet fw4 2>/dev/null || true").stdout or "").strip())
    parts.append("--- openwrt-b inbounds.json ---")
    parts.append(_safe_read_text(openwrt_client_host, helpers.CLIENT_CONFIG_DIR / "inbounds.json"))
    parts.append("--- openwrt-b routing.json ---")
    parts.append(_safe_read_text(openwrt_client_host, helpers.CLIENT_CONFIG_DIR / "routing.json"))
    parts.append("--- openwrt-b xp2p client log ---")
    parts.append((openwrt_client_host.run("cat /tmp/xp2p-client.log 2>/dev/null || true").stdout or "").strip())
    parts.append("--- openwrt-a xp2p server log ---")
    parts.append((openwrt_server_host.run("cat /tmp/xp2p-server.log 2>/dev/null || true").stdout or "").strip())
    parts.append("--- openwrt-b uci dhcp ---")
    parts.append((openwrt_client_host.run("uci show dhcp || true").stdout or "").strip())
    parts.append("--- openwrt-b firewall ---")
    parts.append((openwrt_client_host.run("uci show firewall || true").stdout or "").strip())
    parts.append("--- openwrt-a nslookup ---")
    parts.append((_openwrt_nslookup(openwrt_server_host, C1_FQDN, server=c1_dns_ip).stdout or "").strip())
    parts.append("--- c1 dnsmasq log ---")
    parts.append(
        (alpine_c1_host.run("tail -n 200 /var/log/dnsmasq.log 2>/dev/null || true").stdout or "").strip()
    )
    parts.append("--- c1 netstat ---")
    parts.append(
        (alpine_c1_host.run("netstat -ltnup 2>/dev/null | grep ':53 ' || true").stdout or "").strip()
    )
    parts.append("--- c1 routes ---")
    parts.append((alpine_c1_host.run("ip route show || true").stdout or "").strip())
    parts.append("--- c2 routes ---")
    parts.append((alpine_c2_host.run("ip route show || true").stdout or "").strip())
    return "\n".join(part for part in parts if part)


def _safe_read_text(host, path) -> str:
    if not helpers.path_exists(host, path):
        return ""
    try:
        return (helpers.read_text(host, path) or "").strip()
    except RuntimeError:
        return ""


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
    host.run("for s in $(uci show firewall 2>/dev/null | grep '=redirect' | cut -d. -f2 | cut -d= -f1); do name=$(uci -q get firewall.$s.name); if [ \"$name\" = \"Intercept-DNS\" ]; then uci delete firewall.$s || true; fi; done")
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


def _detect_forward_port(host, target_host: str, target_port: int = 53, role: str = "client") -> str:
    args = ("client", "forward", "list") if role == "client" else ("server", "forward", "list")
    list_res = openwrt_env.run_xp2p_live(host, *args)
    assert list_res.rc == 0, f"forward list failed: {list_res.stderr}"
    output = list_res.stdout or ""
    # Expect a line like: "127.0.0.1:53331  tcp   <target_host>:<port>"
    pattern = rf"127\.0\.0\.1:(\d+)\s+\S+\s+{re.escape(target_host)}:{target_port}"
    match = re.search(pattern, output)
    assert match, f"could not detect forward port in output:\n{output}"
    return match.group(1)


def _assert_dns_response(host, domain: str, expected_ip: str, *, server: str) -> None:
    server_arg = server
    result = openwrt_env.run_guest_script(host, "scripts/linux/nslookup.sh", domain, server_arg)
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
        f"Command: nslookup {domain} {server_arg}\n"
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


def _detect_alpine_ipv4(host) -> str:
    result = openwrt_env.run_alpine_guest_script(host, "scripts/linux/get_primary_ipv4.sh")
    if result.rc != 0:
        pytest.fail(
            "Failed to detect Alpine IPv4 address.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on Alpine host")
    return addresses[0]
