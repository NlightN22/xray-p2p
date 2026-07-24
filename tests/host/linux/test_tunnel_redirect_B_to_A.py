from __future__ import annotations

from contextlib import contextmanager
import time
import pytest

from tests.host import cli_json
from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)
DIAG_IP = "10.77.0.1"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_DOMAIN_IP = "10.77.0.2"
DIAG_DOMAIN_CIDR = f"{DIAG_DOMAIN_IP}/32"
DIAG_DOMAIN = "diag.service.internal"
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE


def _runner(host):
    def _run(*args: str, check: bool = False):
        cmd = list(args)
        pending_targets = {
            ("client", "list"),
            ("client", "forward", "list"),
            ("client", "reverse"),
            ("client", "reverse", "list"),
            ("server", "forward", "list"),
            ("server", "redirect", "list"),
            ("server", "reverse"),
            ("server", "reverse", "list"),
            ("server", "user", "list"),
            ("server", "cert", "state"),
        }
        if "--pending" not in cmd and "-y" not in cmd:
            if cmd[:2] == ["client", "redirect"] and (len(cmd) == 2 or cmd[2].startswith("-")):
                cmd.append("--pending")
            else:
                for target in pending_targets:
                    if tuple(cmd[: len(target)]) == target:
                        cmd.append("--pending")
                        break
        result = linux_env.run_xp2p(host, *cmd)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _wait_for_port(host, port: int, *, timeout_seconds: float = 20.0, interval: float = 0.5) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        check = host.run(f"sudo -n ss -lnt | grep -q ':{port} '")
        if check.rc == 0:
            return
        time.sleep(interval)
    pytest.fail(f"Port {port} did not open on {host.backend.hostname} within {timeout_seconds:.0f}s")


@contextmanager
def _run_sessions(server_host, client_host):
    with linux_env.xp2p_run_session(
        server_host,
        "server",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.SERVER_CONFIG_DIR_NAME,
    ), linux_env.xp2p_run_session(
        client_host,
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
    ):
        _wait_for_port(server_host, 62022)
        _wait_for_port(client_host, 51180)
        yield


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


def _diag_cmd(host, label: str, command: str) -> str:
    result = host.run(command)
    header = f"{label} (rc={result.rc})"
    stdout = result.stdout or ""
    stderr = result.stderr or ""
    return f"{header}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"


def _collect_tunnel_diag(server_host, client_host) -> str:
    sections = []
    targets = [("server", server_host), ("client", client_host)]
    for label, host in targets:
        sections.append(f"== {label} host: {host.backend.hostname} ==")
        sections.append(_diag_cmd(host, "date", "date -Iseconds || date"))
        sections.append(_diag_cmd(host, "processes", "ps -ef | egrep 'xp2p|xray' | egrep -v 'egrep' || true"))
        sections.append(_diag_cmd(host, "sockets", "sudo -n ss -lntp || true"))
        sections.append(_diag_cmd(host, "ls /etc/xp2p", "sudo -n ls -la /etc/xp2p 2>/dev/null || true"))
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state/live/config-client",
                "sudo -n ls -la /etc/xp2p/.state/live/config-client 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state",
                "sudo -n ls -la /etc/xp2p/.state 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state/pending",
                "sudo -n ls -la /etc/xp2p/.state/pending 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state/live/config-server",
                "sudo -n ls -la /etc/xp2p/.state/live/config-server 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state",
                "sudo -n ls -la /etc/xp2p/.state 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /etc/xp2p/.state/pending",
                "sudo -n ls -la /etc/xp2p/.state/pending 2>/dev/null || true",
            )
        )
        sections.append(_diag_cmd(host, "ls /var/log/xp2p", "sudo -n ls -la /var/log/xp2p 2>/dev/null || true"))
        sections.append(
            _diag_cmd(
                host,
                "ls /var/log/xp2p/client",
                "sudo -n ls -la /var/log/xp2p/client 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "ls /var/log/xp2p/server",
                "sudo -n ls -la /var/log/xp2p/server 2>/dev/null || true",
            )
        )
        sections.append(_diag_cmd(host, "ls /tmp", "sudo -n ls -la /tmp 2>/dev/null || true"))
        sections.append(
            _diag_cmd(
                host,
                "cat /tmp/xp2p-client-run.log",
                "sudo -n cat /tmp/xp2p-client-run.log 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /tmp/xp2p-server-run.log",
                "sudo -n cat /tmp/xp2p-server-run.log 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /var/log/xp2p/client/service.log",
                "sudo -n cat /var/log/xp2p/client/service.log 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /var/log/xp2p/server/service.log",
                "sudo -n cat /var/log/xp2p/server/service.log 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /etc/xp2p/.state/live/config-client/inbounds.json",
                "sudo -n cat /etc/xp2p/.state/live/config-client/inbounds.json 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /etc/xp2p/.state/pending/config-client/inbounds.json",
                "sudo -n cat /etc/xp2p/.state/pending/config-client/inbounds.json 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /etc/xp2p/.state/live/config-server/inbounds.json",
                "sudo -n cat /etc/xp2p/.state/live/config-server/inbounds.json 2>/dev/null || true",
            )
        )
        sections.append(
            _diag_cmd(
                host,
                "cat /etc/xp2p/.state/pending/config-server/inbounds.json",
                "sudo -n cat /etc/xp2p/.state/pending/config-server/inbounds.json 2>/dev/null || true",
            )
        )
    return "\n".join(sections)


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_B_to_A(linux_host_factory):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)  # Host A
    client_host = linux_host_factory(linux_env.DEFAULT_SERVER)  # Host B
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup(iface: str | None = None):
        for host in (server_host, client_host):
            host.run("sudo -n pkill -f '/usr/bin/xp2p server run' >/dev/null 2>&1 || true")
            host.run("sudo -n pkill -f '/usr/bin/xp2p client run' >/dev/null 2>&1 || true")
            host.run(f"sudo -n pkill -f {helpers.XRAY_BINARY.as_posix()!r} >/dev/null 2>&1 || true")
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)
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
            "install", "--json",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.parse_json_credential(server_install.stdout or "")
        assert credential["link"], "Expected connection link in server install output"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)
        helpers.assert_reverse_cli_output(
            server_runner,
            "server",
            helpers.INSTALL_ROOT,
            helpers.SERVER_CONFIG_DIR_NAME,
            reverse_tag,
        )

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
        helpers.assert_reverse_cli_output(
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        with _run_sessions(server_host, client_host):
            _ = client_runner(
                "ping",
                DIAG_IP,
                "--tunnel",
                "--count",
                "3",
                check=False,
            )
            initial_domain_ping = client_runner(
                "ping",
                DIAG_DOMAIN,
                "--tunnel",
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

        with _run_sessions(server_host, client_host):
            redirected_ping = client_runner(
                "ping",
                DIAG_IP,
                "--tunnel",
                "--count",
                "3",
                check=False,
            )
            if redirected_ping.rc != 0:
                pytest.fail(
                    "xp2p ping through SOCKS tunnel failed after redirect add.\n"
                    f"STDOUT:\n{redirected_ping.stdout}\nSTDERR:\n{redirected_ping.stderr}\n"
                    f"{_collect_tunnel_diag(server_host, client_host)}"
                )
            assert "0% loss" in _combined_output(redirected_ping)

            domain_before_rule = client_runner(
                "ping",
                DIAG_DOMAIN,
                "--tunnel",
                "--count",
                "3",
                check=False,
            )
            assert domain_before_rule.rc != 0

        redirect_list = client_runner(
            "client",
            "redirect",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--json",
            check=True,
        ).stdout or ""
        assert any(item.get("value") == DIAG_CIDR for item in cli_json.result(redirect_list).get("redirects", []))

        routing = helpers.render_xray(client_host, client_runner, "client", desired=True)
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
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--json",
            check=True,
        ).stdout or ""
        assert any(item.get("value") == DIAG_DOMAIN for item in cli_json.result(redirect_list).get("redirects", []))

        routing = helpers.render_xray(client_host, client_runner, "client", desired=True)
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

        routing_after_domain_removal = helpers.render_xray(client_host, client_runner, "client", desired=True)
        helpers.assert_redirect_rule(routing_after_domain_removal, DIAG_CIDR, helpers.expected_proxy_tag(SERVER_IP))
        helpers.assert_no_domain_redirect_rule(
            routing_after_domain_removal, DIAG_DOMAIN, helpers.expected_proxy_tag(SERVER_IP)
        )

        with _run_sessions(server_host, client_host):
            redirected_ping_after_domain = client_runner(
                "ping",
                DIAG_IP,
                "--tunnel",
                "--count",
                "3",
                check=False,
            )
            if redirected_ping_after_domain.rc != 0:
                pytest.fail(
                    "xp2p ping through SOCKS tunnel failed after domain removal.\n"
                    f"STDOUT:\n{redirected_ping_after_domain.stdout}\nSTDERR:\n{redirected_ping_after_domain.stderr}\n"
                    f"{_collect_tunnel_diag(server_host, client_host)}"
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
            "--host",
            SERVER_IP,
            check=True,
        )

        routing_after_remove = helpers.render_xray(client_host, client_runner, "client", desired=True)
        helpers.assert_no_redirect_rule(routing_after_remove, DIAG_CIDR)
        helpers.assert_no_domain_redirect_rule(routing_after_remove, DIAG_DOMAIN)

        final_list = client_runner(
            "client",
            "redirect",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert DIAG_CIDR not in final_list
        assert DIAG_DOMAIN_CIDR not in final_list
    finally:
        cleanup(iface_name)
