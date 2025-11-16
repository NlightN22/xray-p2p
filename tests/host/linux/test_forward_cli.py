from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers

CLIENT_INBOUNDS = helpers.CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"
FORWARD_BASE_PORT = 53331
SERVER_INSTALL_HOST = "forward-server.example"


def _install_client(runner) -> None:
    runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--server-address",
        "10.77.55.10",
        "--user",
        "forwarder@example.com",
        "--password",
        "forward-pass",
        "--force",
        check=True,
    )


def _install_server(runner) -> None:
    runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        "62090",
        "--host",
        SERVER_INSTALL_HOST,
        "--force",
        check=True,
    )


def _forward_cmd(runner, role: str, subcommand: str, config_dir: str, *extra: str, check: bool = False):
    args = [
        role,
        "forward",
        subcommand,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        config_dir,
    ]
    args.extend(extra)
    return runner(*args, check=check)


def _parse_forward_list(output: str) -> list[dict[str, str]]:
    lines = [line.rstrip() for line in (output or "").splitlines() if line.strip()]
    if not lines:
        return []
    lowered = [line.lower() for line in lines]
    if any(line.startswith("no forward rules configured") for line in lowered):
        return []
    start_idx = 0
    for idx, line in enumerate(lowered):
        if line.startswith("listen"):
            start_idx = idx + 1
            break
    rows: list[dict[str, str]] = []
    for line in lines[start_idx:]:
        parts = [part for part in line.split() if part]
        if len(parts) < 4:
            continue
        rows.append(
            {
                "listen": parts[0],
                "protocols": parts[1],
                "target": parts[2],
                "remark": " ".join(parts[3:]),
            }
        )
    return rows


def _assert_forward_list_empty(runner, role: str, config_dir: str) -> None:
    result = _forward_cmd(runner, role, "list", config_dir, check=True)
    output = (result.stdout or "").strip()
    assert "no forward rules configured" in output.lower()
    assert _parse_forward_list(output) == []


def _assert_list_contains(rows: list[dict[str, str]], listen: int, target: str, remark: str, protocols: str) -> None:
    formatted_listen = f"127.0.0.1:{listen}"
    formatted_target = f"{target}"
    formatted_protocols = protocols
    formatted_remark = remark
    for row in rows:
        if (
            row.get("listen") == formatted_listen
            and row.get("target") == formatted_target
            and row.get("remark") == formatted_remark
            and row.get("protocols") == formatted_protocols
        ):
            return
    pytest.fail(f"Forward list does not contain {formatted_listen} -> {formatted_target}")


@pytest.mark.host
@pytest.mark.linux
def test_client_forward_cli_flow(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    try:
        _install_client(xp2p_client_runner)
        _assert_forward_list_empty(xp2p_client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME)

        first_target_ip = "203.0.113.50"
        first_target_port = 22
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "add",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--target",
            f"{first_target_ip}:{first_target_port}",
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        forwards = client_state.get("forwards") or []
        first_entry = helpers.assert_forward_rule_entry(
            forwards,
            listen_port=FORWARD_BASE_PORT,
            listen_address="127.0.0.1",
            target_ip=first_target_ip,
            target_port=first_target_port,
            protocol="both",
        )
        first_port = int(first_entry.get("listen_port", 0))
        assert first_port == FORWARD_BASE_PORT
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            client_inbounds,
            first_port,
            listen_address="127.0.0.1",
            target_ip=first_target_ip,
            target_port=first_target_port,
            protocol="both",
        )
        list_rows = _parse_forward_list(
            _forward_cmd(xp2p_client_runner, "client", "list", helpers.CLIENT_CONFIG_DIR_NAME, check=True).stdout or ""
        )
        _assert_list_contains(
            list_rows,
            first_port,
            f"{first_target_ip}:{first_target_port}",
            helpers.expected_forward_remark(first_target_ip, first_target_port),
            "tcp,udp",
        )

        explicit_port = 61080
        explicit_target_ip = "198.51.100.25"
        explicit_target_port = 443
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "add",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--listen-port",
            str(explicit_port),
            "--target",
            f"{explicit_target_ip}:{explicit_target_port}",
            "--proto",
            "tcp",
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        forwards = client_state.get("forwards") or []
        explicit_entry = helpers.assert_forward_rule_entry(
            forwards,
            listen_port=explicit_port,
            listen_address="127.0.0.1",
            target_ip=explicit_target_ip,
            target_port=explicit_target_port,
            protocol="tcp",
        )
        explicit_tag = explicit_entry.get("tag") or helpers.expected_forward_tag(explicit_port)
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            client_inbounds,
            explicit_port,
            listen_address="127.0.0.1",
            target_ip=explicit_target_ip,
            target_port=explicit_target_port,
            protocol="tcp",
        )

        second_target_ip = "198.51.100.60"
        second_target_port = 3389
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "add",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--target",
            f"{second_target_ip}:{second_target_port}",
            "--proto",
            "udp",
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        forwards = client_state.get("forwards") or []
        second_entry = forwards[-1]
        second_port = int(second_entry.get("listen_port") or 0)
        assert second_port > first_port
        helpers.assert_forward_rule_entry(
            forwards,
            listen_port=second_port,
            listen_address="127.0.0.1",
            target_ip=second_target_ip,
            target_port=second_target_port,
            protocol="udp",
        )
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            client_inbounds,
            second_port,
            listen_address="127.0.0.1",
            target_ip=second_target_ip,
            target_port=second_target_port,
            protocol="udp",
        )
        list_rows = _parse_forward_list(
            _forward_cmd(xp2p_client_runner, "client", "list", helpers.CLIENT_CONFIG_DIR_NAME, check=True).stdout or ""
        )
        assert len(list_rows) == 3

        # Remove via listen-port.
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "remove",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--listen-port",
            str(first_port),
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        helpers.assert_no_forward_rule_entry(client_state.get("forwards") or [], first_port)
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(client_inbounds, first_port)

        # Remove via tag.
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "remove",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--tag",
            explicit_tag,
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        helpers.assert_no_forward_rule_entry(client_state.get("forwards") or [], explicit_port)
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(client_inbounds, explicit_port)

        # Remove via remark.
        second_remark = second_entry.get("remark") or helpers.expected_forward_remark(
            second_target_ip, second_target_port
        )
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "remove",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--remark",
            second_remark,
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        helpers.assert_no_forward_rule_entry(client_state.get("forwards") or [], second_port)
        client_inbounds = helpers.read_json(client_host, CLIENT_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(client_inbounds, second_port)
        _assert_forward_list_empty(xp2p_client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME)
    finally:
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_forward_cli_flow(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    try:
        _install_server(xp2p_server_runner)
        _assert_forward_list_empty(xp2p_server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME)

        first_target_ip = "203.0.113.75"
        first_target_port = 25
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "add",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--target",
            f"{first_target_ip}:{first_target_port}",
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        forwards = server_state.get("forward_rules") or []
        first_entry = helpers.assert_forward_rule_entry(
            forwards,
            listen_port=FORWARD_BASE_PORT,
            listen_address="127.0.0.1",
            target_ip=first_target_ip,
            target_port=first_target_port,
            protocol="both",
        )
        first_port = int(first_entry.get("listen_port", 0))
        assert first_port == FORWARD_BASE_PORT
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            server_inbounds,
            first_port,
            listen_address="127.0.0.1",
            target_ip=first_target_ip,
            target_port=first_target_port,
            protocol="both",
        )

        explicit_port = 61100
        explicit_target_ip = "198.51.100.85"
        explicit_target_port = 80
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "add",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--listen-port",
            str(explicit_port),
            "--target",
            f"{explicit_target_ip}:{explicit_target_port}",
            "--proto",
            "tcp",
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        forwards = server_state.get("forward_rules") or []
        explicit_entry = helpers.assert_forward_rule_entry(
            forwards,
            listen_port=explicit_port,
            listen_address="127.0.0.1",
            target_ip=explicit_target_ip,
            target_port=explicit_target_port,
            protocol="tcp",
        )
        explicit_tag = explicit_entry.get("tag") or helpers.expected_forward_tag(explicit_port)
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            server_inbounds,
            explicit_port,
            listen_address="127.0.0.1",
            target_ip=explicit_target_ip,
            target_port=explicit_target_port,
            protocol="tcp",
        )

        second_target_ip = "198.51.100.95"
        second_target_port = 1194
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "add",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--target",
            f"{second_target_ip}:{second_target_port}",
            "--proto",
            "udp",
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        forwards = server_state.get("forward_rules") or []
        second_entry = forwards[-1]
        second_port = int(second_entry.get("listen_port") or 0)
        assert second_port > first_port
        helpers.assert_forward_rule_entry(
            forwards,
            listen_port=second_port,
            listen_address="127.0.0.1",
            target_ip=second_target_ip,
            target_port=second_target_port,
            protocol="udp",
        )
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_forward_inbound_entry(
            server_inbounds,
            second_port,
            listen_address="127.0.0.1",
            target_ip=second_target_ip,
            target_port=second_target_port,
            protocol="udp",
        )

        # Remove via listen-port.
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "remove",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--listen-port",
            str(first_port),
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        helpers.assert_no_forward_rule_entry(server_state.get("forward_rules") or [], first_port)
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(server_inbounds, first_port)

        # Remove via tag.
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "remove",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--tag",
            explicit_tag,
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        helpers.assert_no_forward_rule_entry(server_state.get("forward_rules") or [], explicit_port)
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(server_inbounds, explicit_port)

        # Remove via remark.
        second_remark = second_entry.get("remark") or helpers.expected_forward_remark(
            second_target_ip, second_target_port
        )
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "remove",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--remark",
            second_remark,
            check=True,
        )
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        helpers.assert_no_forward_rule_entry(server_state.get("forward_rules") or [], second_port)
        server_inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        helpers.assert_no_forward_inbound_entry(server_inbounds, second_port)
        _assert_forward_list_empty(xp2p_server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME)
    finally:
        helpers.cleanup_server_install(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_forward_add_warns_without_redirect(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    try:
        _install_client(xp2p_client_runner)
        result = _forward_cmd(
            xp2p_client_runner,
            "client",
            "add",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--target",
            "198.51.100.200:8080",
            check=True,
        )
        stderr = (result.stderr or "").lower()
        assert "xp2p client forward has no matching redirect" in stderr
        forwards = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES).get("forwards") or []
        assert forwards, "Expected client forward entry recorded"
        listen_port = forwards[-1].get("listen_port")
        assert listen_port, "Client forward listen port missing from state"
        _forward_cmd(
            xp2p_client_runner,
            "client",
            "remove",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--listen-port",
            str(listen_port),
            check=True,
        )
    finally:
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_forward_add_warns_without_redirect(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    try:
        _install_server(xp2p_server_runner)
        result = _forward_cmd(
            xp2p_server_runner,
            "server",
            "add",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--target",
            "203.0.113.200:993",
            check=True,
        )
        stderr = (result.stderr or "").lower()
        assert "xp2p server forward has no matching redirect" in stderr
        forwards = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES).get("forward_rules") or []
        assert forwards, "Expected server forward entry recorded"
        listen_port = forwards[-1].get("listen_port")
        assert listen_port, "Server forward listen port missing from state"
        _forward_cmd(
            xp2p_server_runner,
            "server",
            "remove",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--listen-port",
            str(listen_port),
            check=True,
        )
    finally:
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
