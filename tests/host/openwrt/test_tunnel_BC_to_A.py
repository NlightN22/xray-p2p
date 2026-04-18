from __future__ import annotations

import re
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_B_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
CLIENT_C_MACHINE = openwrt_env.OPENWRT_MACHINES[2]
SERVER_IP = "10.63.30.11"
ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")
REQUIRED_LIVE_ARTIFACTS = ("runtime.json", "xray.json")


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p_live(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _extract_link(output: str) -> str:
    for raw in (output or "").splitlines():
        stripped = raw.strip()
        if stripped.startswith("trojan://"):
            return stripped
    pytest.fail(f"xp2p server user add did not emit trojan link.\nSTDOUT:\n{output}")


def _install_client(host, runner, link: str):
    helpers.cleanup_client_install(host, runner)
    runner(
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


def _apply_pending_config(host, role: str) -> None:
    helpers.state_pending_config(host, role)


def _apply_pending_config_wait(host, role: str) -> None:
    _apply_pending_config(host, role)


def _wait_for_live_xray_configs(
    host,
    config_dir,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if config_dir == helpers.CLIENT_CONFIG_DIR:
            live_dir = helpers.CLIENT_LIVE_DIR
        elif config_dir == helpers.SERVER_CONFIG_DIR:
            live_dir = helpers.SERVER_LIVE_DIR
        else:
            raise ValueError(f"Unsupported config dir: {config_dir}")
        missing = [
            (live_dir / name).as_posix()
            for name in REQUIRED_LIVE_ARTIFACTS
            if not helpers.path_exists_live(host, live_dir / name)
        ]
        if not missing:
            return
        time.sleep(interval)
    raise AssertionError(
        f"Live xray configs did not appear on {host.backend.hostname}: {missing}"
    )


def _pending_config_present(host, role: str) -> bool:
    if role == "client":
        pending_path = helpers.CONFIG_PENDING_ROOT / "xp2p-client.toml"
    elif role == "server":
        pending_path = helpers.CONFIG_PENDING_ROOT / "xp2p-server.toml"
    else:
        raise ValueError(f"Unsupported role: {role}")
    return helpers.path_exists_exact(host, pending_path)

def _strip_ansi(value: str | None) -> str:
    if not value:
        return ""
    return ANSI_ESCAPE_RE.sub("", value)


def _extract_client_users(output: str) -> set[str]:
    cleaned = _strip_ansi(output)
    users: set[str] = set()
    for raw_line in cleaned.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("TAG"):
            continue
        if not line.startswith("proxy-"):
            continue
        columns = [segment.strip() for segment in re.split(r"\s{2,}", line) if segment.strip()]
        if len(columns) >= 7:
            users.add(columns[6])
    return users


def _assert_server_state_reports_user(
    host,
    expected_user: str,
    *,
    attempts: int = 10,
    delay_seconds: float = 3.0,
):
    install_path = helpers.INSTALL_ROOT.as_posix()
    last_stdout = ""
    for _ in range(attempts):
        result = openwrt_env.run_xp2p_live(
            host,
            "server",
            "state",
            "--path",
            install_path,
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p server state --once failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        last_stdout = result.stdout or ""
        users = _extract_client_users(last_stdout)
        if expected_user in users:
            return
        time.sleep(delay_seconds)
    pytest.fail(
        f"xp2p server state never reported user {expected_user} after {attempts} attempts.\n"
        f"Last output:\n{last_stdout}"
    )


def _assert_server_state_reports_users(
    host,
    expected_users: set[str],
    *,
    attempts: int = 10,
    delay_seconds: float = 3.0,
):
    install_path = helpers.INSTALL_ROOT.as_posix()
    expected = {user.strip().lower() for user in expected_users if user.strip()}
    if not expected:
        pytest.fail("expected_users is empty")
    last_stdout = ""
    for _ in range(attempts):
        result = openwrt_env.run_xp2p_live(
            host,
            "server",
            "state",
            "--path",
            install_path,
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p server state --once failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        last_stdout = result.stdout or ""
        users = {user.strip().lower() for user in _extract_client_users(last_stdout)}
        if expected.issubset(users):
            return
        time.sleep(delay_seconds)
    pytest.fail(
        "xp2p server state did not report all expected users "
        f"{sorted(expected)} after {attempts} attempts.\nLast output:\n{last_stdout}"
    )


def _assert_server_state_reports_users_alive(
    host,
    expected_users: set[str],
    *,
    attempts: int = 10,
    delay_seconds: float = 3.0,
):
    install_path = helpers.INSTALL_ROOT.as_posix()
    expected = {user.strip().lower() for user in expected_users if user.strip()}
    if not expected:
        pytest.fail("expected_users is empty")
    last_stdout = ""
    for _ in range(attempts):
        result = openwrt_env.run_xp2p_live(
            host,
            "server",
            "state",
            "--path",
            install_path,
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p server state --once failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        last_stdout = result.stdout or ""
        rows = tunnel_common.parse_state_rows(last_stdout)
        alive_users = {
            (row.get("CLIENT_USER") or "").strip().lower()
            for row in rows
            if (row.get("STATUS") or "").strip().lower() == "alive"
        }
        if expected.issubset(alive_users):
            return
        time.sleep(delay_seconds)
    pytest.fail(
        "xp2p server state did not report all expected users as alive "
        f"{sorted(expected)} after {attempts} attempts.\nLast output:\n{last_stdout}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_BC_to_A(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_b = openwrt_host_factory(CLIENT_B_MACHINE)
    client_c = openwrt_host_factory(CLIENT_C_MACHINE)

    for machine, host in (
        (SERVER_MACHINE, server_host),
        (CLIENT_B_MACHINE, client_b),
        (CLIENT_C_MACHINE, client_c),
    ):
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    server_runner = _runner(server_host)
    client_b_runner = _runner(client_b)
    client_c_runner = _runner(client_c)

    helpers.cleanup_server_install(server_host, server_runner)
    helpers.cleanup_client_install(client_b, client_b_runner)
    helpers.cleanup_client_install(client_c, client_c_runner)
    helpers.remove_path(server_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
    for host in (client_b, client_c):
        helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)
    for host in (server_host, client_b, client_c):
        openwrt_env._stop_xp2p_services(host)
        openwrt_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        host.run("rm -f /tmp/xp2p-*.log >/dev/null 2>&1 || true")
    helpers.dump_install_dirs(server_host, "tunnel BC to A after cleanup")
    helpers.dump_install_dirs(client_b, "tunnel BC to A after cleanup")
    helpers.dump_install_dirs(client_c, "tunnel BC to A after cleanup")
    helpers.dump_apply_dirs(server_host, "tunnel BC to A after cleanup")
    helpers.dump_apply_dirs(client_b, "tunnel BC to A after cleanup")
    helpers.dump_apply_dirs(client_c, "tunnel BC to A after cleanup")

    try:
        server_install = server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--port",
            "62070",
            "--force",
            check=True,
        )
        helpers.dump_apply_dirs(server_host, "tunnel BC to A after server install")
        default_cred = helpers.extract_trojan_credential(server_install.stdout or "")
        reverse_default = helpers.expected_reverse_tag(default_cred["user"], SERVER_IP)
        if _pending_config_present(server_host, "server"):
            _apply_pending_config_wait(server_host, "server")
        helpers.wait_for_live_config(server_host, "server")
        _wait_for_live_xray_configs(server_host, helpers.SERVER_CONFIG_DIR)

        user_add = server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "client-two@example.com",
            "--password",
            "client-two-pass",
            "--host",
            SERVER_IP,
            check=True,
        )
        second_link = _extract_link(user_add.stdout or "")
        reverse_second = helpers.expected_reverse_tag("client-two@example.com", SERVER_IP)

        server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "client-norev@example.com",
            "--password",
            "client-norev-pass",
            "--host",
            SERVER_IP,
            "--no-reverse",
            check=True,
        )
        reverse_norev = helpers.expected_reverse_tag("client-norev@example.com", SERVER_IP)

        _apply_pending_config_wait(server_host, "server")
        helpers.wait_for_live_config(server_host, "server")
        _wait_for_live_xray_configs(server_host, helpers.SERVER_CONFIG_DIR)
        server_state = helpers.read_live_server_config(server_host)
        server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        for reverse_tag, user in (
            (reverse_default, default_cred["user"]),
            (reverse_second, "client-two@example.com"),
    ):
            helpers.assert_server_reverse_state(
                server_state,
                reverse_tag,
                user=user,
                host=SERVER_IP,
            )
            helpers.assert_server_reverse_routing(server_routing, reverse_tag, user=user)
        recorded_server_tags = set((server_state.get("reverse_channels") or {}).keys())
        assert recorded_server_tags == {reverse_default, reverse_second}
        assert reverse_norev not in recorded_server_tags
        for reverse_tag in (reverse_default, reverse_second):
            helpers.assert_reverse_cli_output_live(
                server_runner,
                "server",
                helpers.INSTALL_ROOT,
                helpers.SERVER_CONFIG_DIR_NAME,
                reverse_tag,
            )

        _install_client(client_b, client_b_runner, default_cred["link"])
        _install_client(client_c, client_c_runner, second_link)
        helpers.dump_apply_dirs(client_b, "tunnel BC to A after client B install")
        helpers.dump_apply_dirs(client_c, "tunnel BC to A after client C install")

        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        _apply_pending_config_wait(client_b, "client")
        helpers.wait_for_live_config(client_b, "client")
        _wait_for_live_xray_configs(client_b, helpers.CLIENT_CONFIG_DIR)
        client_b_state = helpers.read_live_client_config(client_b)
        client_b_routing = helpers.read_live_json(client_b, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_client_reverse_artifacts(client_b_routing, reverse_default, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_b_state,
            reverse_default,
            endpoint_tag=endpoint_tag,
            user=default_cred["user"],
            host=SERVER_IP,
        )
        assert set((client_b_state.get("reverse") or {}).keys()) == {reverse_default}
        helpers.assert_reverse_cli_output_live(
            client_b_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_default,
            )

        _apply_pending_config_wait(client_c, "client")
        helpers.wait_for_live_config(client_c, "client")
        _wait_for_live_xray_configs(client_c, helpers.CLIENT_CONFIG_DIR)
        client_c_state = helpers.read_live_client_config(client_c)
        client_c_routing = helpers.read_live_json(client_c, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_client_reverse_artifacts(client_c_routing, reverse_second, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_c_state,
            reverse_second,
            endpoint_tag=endpoint_tag,
            user="client-two@example.com",
            host=SERVER_IP,
        )
        assert set((client_c_state.get("reverse") or {}).keys()) == {reverse_second}
        helpers.assert_reverse_cli_output_live(
            client_c_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_second,
        )

        redirect_domains: list[dict[str, str]] = []
        try:
            for reverse_tag in (reverse_default, reverse_second):
                domain = f"full:{reverse_tag}"
                server_runner(
                    "server",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    domain,
                    "--tag",
                    reverse_tag,
                    check=True,
                )
                redirect_domains.append({"domain": domain, "tag": reverse_tag})
                _apply_pending_config_wait(server_host, "server")
                list_output = server_runner(
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert domain in list_output.lower(), f"Server redirect list missing {domain}"
                server_state = helpers.read_live_server_config(server_host)
                server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
                helpers.assert_server_redirect_state(server_state, domain, reverse_tag)
                helpers.assert_server_redirect_rule(server_routing, domain, reverse_tag)

            try:
                helpers.ensure_service_running(server_host, "server")
                helpers.ensure_service_running(client_b, "client")
                helpers.ensure_service_running(client_c, "client")
                helpers.wait_for_apply_request_clear(server_host, timeout_seconds=60.0)
                helpers.wait_for_apply_request_clear(client_b, timeout_seconds=60.0)
                helpers.wait_for_apply_request_clear(client_c, timeout_seconds=60.0)
                helpers.wait_for_live_config(server_host, "server")
                helpers.wait_for_live_config(client_b, "client")
                helpers.wait_for_live_config(client_c, "client")
                helpers.wait_for_heartbeat_state(
                    server_host,
                    path=helpers.SERVER_HEARTBEAT_STATE_FILE,
                )
                _assert_server_state_reports_user(server_host, default_cred["user"])
                helpers.wait_for_heartbeat_state(
                    server_host,
                    path=helpers.SERVER_HEARTBEAT_STATE_FILE,
                )
                _assert_server_state_reports_users(
                    server_host,
                    {default_cred["user"], "client-two@example.com"},
                )
                _assert_server_state_reports_users_alive(
                    server_host,
                    {default_cred["user"], "client-two@example.com"},
                )
                for runner, origin in ((client_b_runner, "client-b"), (client_c_runner, "client-c")):
                    result = runner(
                        "ping",
                        SERVER_IP,
                        "--tunnel",
                        "--count",
                        "3",
                        check=True,
                    )
                    stdout = (result.stdout or "").lower()
                    assert "0% loss" in stdout, (
                        f"xp2p ping from {origin} did not report zero loss:\n"
                        f"{result.stdout}"
                    )
                helpers.wait_for_heartbeat_state(
                    server_host,
                    path=helpers.SERVER_HEARTBEAT_STATE_FILE,
                )
                _assert_server_state_reports_user(server_host, "client-two@example.com")
            except BaseException:
                helpers.dump_logs(server_host, "tunnel BC to A server")
                helpers.dump_logs(client_b, "tunnel BC to A client B")
                helpers.dump_logs(client_c, "tunnel BC to A client C")
                raise
        finally:
            while redirect_domains:
                entry = redirect_domains.pop()
                domain = entry["domain"]
                tag = entry["tag"]
                list_output = server_runner(
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                ).stdout or ""
                listed = (list_output or "").lower()
                if domain not in listed:
                    continue
                removal = server_runner(
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    domain,
                    "--tag",
                    tag,
                    check=False,
                )
                stderr = (removal.stderr or "").lower()
                if removal.rc != 0 and "not found" not in stderr:
                    pytest.fail(
                        f"Failed to remove redirect {domain}:\nSTDOUT:\n{removal.stdout}\nSTDERR:\n{removal.stderr}"
                    )
            _apply_pending_config_wait(server_host, "server")
            final_list = server_runner(
                "server",
                "redirect",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=True,
            ).stdout or ""
            assert "no server redirect rules configured" in final_list.lower()
    finally:
        helpers.cleanup_client_install(client_b, client_b_runner)
        helpers.cleanup_client_install(client_c, client_c_runner)
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.remove_path(server_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
        for host in (client_b, client_c):
            helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)
