from __future__ import annotations

import time

import pytest

from tests.host import cli_json
from tests.host.linux import _helpers as helpers
from tests.host.linux import _identity_ldap
from tests.host.linux import env as linux_env
from tests.host.linux.test_identity_sync_e2e import (
    _append_server_config,
    _identity_state,
    _ldap_provider_config,
)
from tests.host.linux.test_tunnel_BC_to_A import _extract_client_users, _runner, _wait_for_port

SERVER_IP = "10.62.10.12"
CLIENT_C_ALIAS = "198.19.77.77"
CLIENT_DIAG_PORT = "62023"
SERVER_SERVICE_PORT = "62080"
CLIENT_SOCKS = "127.0.0.1:51180"


@pytest.mark.host
@pytest.mark.linux
def test_identity_group_acl_allows_service_tunnel_to_identity_client(
    linux_host_factory, aux_host, ldap_directory
):
    server_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    client_b = linux_host_factory(linux_env.DEFAULT_CLIENT)
    client_c = aux_host
    server_runner = _runner(server_host)
    client_b_runner = _runner(client_b)
    client_c_runner = _runner(client_c)

    _add_ip_alias(client_c, f"{CLIENT_C_ALIAS}/32")
    try:
        server_runner(
            "server",
            "install", "--json",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--port",
            SERVER_SERVICE_PORT,
            "--force",
            check=True,
        )
        _append_server_config(server_host, _ldap_provider_config())
        server_runner("server", "identity", "sync", check=True)
        state = _identity_state(server_host)
        alice_label = state["current"]["subjects"]["usr-10001"]["user_label"]
        carol_label = state["current"]["subjects"]["usr-10003"]["user_label"]
        alice_link = _provision_identity_link(server_runner, alice_label)
        carol_link = _provision_identity_link(server_runner, carol_label)
        reverse_carol = helpers.expected_reverse_tag(carol_label, SERVER_IP)

        _install_client(client_b_runner, alice_link)
        _install_client(client_c_runner, carol_link)
        _add_cidr_group_redirect(server_runner, f"{CLIENT_C_ALIAS}/32", reverse_carol, "engineering")

        _start_service(server_host, server_runner, "server", 62022)
        _start_service(client_b, client_b_runner, "client", int(CLIENT_DIAG_PORT))
        _start_service(client_c, client_c_runner, "client", int(CLIENT_DIAG_PORT))
        _wait_for_apply_request_clear(server_host)
        _wait_for_apply_request_clear(client_b)
        _wait_for_apply_request_clear(client_c)
        _assert_server_state_users(server_host, {alice_label, carol_label})

        _assert_diag_http_reachable(client_b, CLIENT_C_ALIAS)

        _identity_ldap.run_ldap(aux_host, "set-membership", "alice", "engineering", "absent")
        server_runner("server", "identity", "sync", check=True)
        _assert_diag_http_blocked(client_b, CLIENT_C_ALIAS)
    finally:
        for runner in (client_b_runner, client_c_runner, server_runner):
            runner("client", "service", "stop", check=False)
            runner("server", "service", "stop", check=False)
        _remove_ip_alias(client_c, f"{CLIENT_C_ALIAS}/32")


def _provision_identity_link(server_runner, label: str) -> str:
    result = server_runner(
        "server", "identity", "provision", label, "--host", SERVER_IP, "--json", check=True
    )
    return cli_json.link(result.stdout or "")


def _install_client(client_runner, link: str) -> None:
    client_runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--link",
        link,
        "--force",
        check=True,
    )


def _add_cidr_group_redirect(server_runner, cidr: str, tag: str, group: str) -> None:
    server_runner(
        "server",
        "redirect",
        "add",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--cidr",
        cidr,
        "--tag",
        tag,
        "--access",
        "restricted",
        "--allow-group",
        group,
        check=True,
    )


def _start_service(host, runner, role: str, port: int) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    runner(role, "service", "start", check=True)
    _wait_for_service_active(runner, role)
    _wait_for_port(host, port)


def _wait_for_service_active(runner, role: str) -> None:
    deadline = time.time() + 60.0
    while time.time() < deadline:
        if runner(role, "service", "status").rc == 0:
            return
        time.sleep(1.0)
    pytest.fail(f"xp2p {role} service did not become active")


def _wait_for_apply_request_clear(host) -> None:
    request_path = helpers.STATE_ROOT / "apply.request"
    deadline = time.time() + 90.0
    while time.time() < deadline:
        if not linux_env.path_exists(host, request_path):
            return
        time.sleep(1.0)
    pytest.fail(f"{request_path} did not clear on {host.backend.hostname}")


def _assert_server_state_users(host, expected_users: set[str]) -> None:
    expected = {user.lower() for user in expected_users}
    last_stdout = ""
    for _ in range(20):
        result = linux_env.run_xp2p(host, "server", "state", "--json", "--path", helpers.INSTALL_ROOT.as_posix())
        if result.rc != 0:
            pytest.fail(f"xp2p server state failed.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")
        last_stdout = result.stdout or ""
        users = {user.lower() for user in _extract_client_users(last_stdout)}
        if expected.issubset(users):
            return
        time.sleep(2.0)
    pytest.fail(f"server state did not report {sorted(expected)}.\nLast output:\n{last_stdout}")


def _assert_diag_http_reachable(host, target: str) -> None:
    for _ in range(10):
        result = _curl_diag(host, target)
        if result.rc == 0 and (result.stdout or "").strip() != "000":
            return
        time.sleep(1.0)
    pytest.fail(f"diagnostics port was not reachable.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")


def _assert_diag_http_blocked(host, target: str) -> None:
    result = _curl_diag(host, target)
    assert result.rc != 0 or (result.stdout or "").strip() == "000"


def _curl_diag(host, target: str):
    url = f"https://{target}:{CLIENT_DIAG_PORT}/control/v1/ping"
    return host.run("curl -ksS -o /dev/null -w '%{http_code}' --max-time 5 " f"--socks5-hostname {CLIENT_SOCKS} {url}")


def _add_ip_alias(host, cidr: str) -> None:
    result = host.run(f"sudo -n ip addr add {cidr} dev lo")
    if result.rc != 0 and "file exists" not in ((result.stderr or "") + (result.stdout or "")).lower():
        pytest.fail(f"Failed to add alias {cidr}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")


def _remove_ip_alias(host, cidr: str) -> None:
    host.run(f"sudo -n ip addr del {cidr} dev lo >/dev/null 2>&1 || true")
