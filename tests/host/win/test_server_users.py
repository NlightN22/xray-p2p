import re

import pytest

import time

from tests.host.win import env as _env

from .test_server_install import (
    SERVER_CONFIG_DIR_NAME,
    SERVER_INBOUNDS,
    SERVER_INSTALL_DIR,
    SERVER_STATE_FILES,
    _read_remote_json,
    _trojan_inbound,
)


def _trojan_clients(data: dict) -> list[dict]:
    trojan = _trojan_inbound(data)
    settings = trojan.get("settings", {})
    assert isinstance(settings, dict), "Expected trojan settings to be a dictionary"
    clients = settings.get("clients", [])
    assert isinstance(clients, list), "Expected trojan clients to be a list"
    return clients


def _initial_install_client(server_host) -> dict:
    current = _read_remote_json(server_host, SERVER_INBOUNDS)
    clients = _trojan_clients(current)
    assert len(clients) == 1, "xp2p server install should provision a single default client"
    default = clients[0]
    assert isinstance(default.get("email"), str) and default["email"].startswith("client-")
    assert isinstance(default.get("password"), str) and default["password"]
    return default


def _remove_initial_install_client(server_host, xp2p_server_runner):
    default_client = _initial_install_client(server_host)
    xp2p_server_runner(
        "server",
        "user",
        "remove",
        "--path",
        str(SERVER_INSTALL_DIR),
        "--config-dir",
        SERVER_CONFIG_DIR_NAME,
        "--id",
        default_client["email"],
        check=True,
    )
    cleared = _read_remote_json(server_host, SERVER_INBOUNDS)
    assert _trojan_clients(cleared) == []
    return default_client


def _wait_for_apply_request_clear(host, *, timeout: float = 60.0) -> None:
    apply_path = _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request"
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not _env.path_exists(host, apply_path):
            return
        time.sleep(1.0)
    pytest.fail(f"apply.request did not clear after {timeout} seconds.")


def _ensure_live_inbounds(server_host, xp2p_server_runner, xp2p_server_run_factory) -> None:
    live_inbounds = SERVER_INBOUNDS
    if _env.path_exists(server_host, live_inbounds):
        return
    if xp2p_server_run_factory is not None:
        with xp2p_server_run_factory(str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME) as session:
            assert session["pid"] > 0
    else:
        if not _env.service_exists(server_host, "xp2p-server"):
            pytest.skip("xp2p-server service is not registered; MSI install required.")
        xp2p_server_runner("server", "service", "start", check=True)
        _wait_for_apply_request_clear(server_host, timeout=90.0)
        xp2p_server_runner("server", "service", "stop", check=True)
    deadline = time.time() + 30.0
    while time.time() < deadline:
        if _env.path_exists(server_host, live_inbounds):
            break
        time.sleep(1.0)
    if not _env.path_exists(server_host, live_inbounds):
        pytest.fail("Live inbounds.json was not created after apply request.")


def _is_unreserved(value: str) -> bool:
    return re.fullmatch(r"[A-Za-z0-9._~-]+", value or "") is not None


def _link_host(server_host) -> str:
    return _env.get_host_ipv4(server_host)


@pytest.mark.host
@pytest.mark.win
def test_server_install_creates_and_allows_removing_default_user(
    server_host, xp2p_server_runner, xp2p_server_run_factory, xp2p_msi_path
):
    _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62030",
            "--force",
            check=True,
            )
        _ensure_live_inbounds(server_host, xp2p_server_runner, xp2p_server_run_factory)

        default_client = _initial_install_client(server_host)
        assert default_client["email"].startswith("client-")

        _remove_initial_install_client(server_host, xp2p_server_runner)
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_user_add_and_idempotent(
    server_host, xp2p_server_runner, xp2p_server_run_factory, xp2p_msi_path
):
    _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        link_host = _link_host(server_host)
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62031",
            "--force",
            check=True,
            )
        _ensure_live_inbounds(server_host, xp2p_server_runner, xp2p_server_run_factory)

        _remove_initial_install_client(server_host, xp2p_server_runner)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            )

        first_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        first_clients = _trojan_clients(first_inbounds)
        assert len(first_clients) == 1
        assert first_clients[0].get("email") == "alpha"
        assert first_clients[0].get("password") == "secret-one"

        duplicate = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            )
        assert duplicate.rc != 0, "Expected failure when adding duplicate user without --force"

        second_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        second_clients = _trojan_clients(second_inbounds)
        assert len(second_clients) == 1
        assert second_clients[0].get("password") == "secret-one"

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "alpha",
            "--password",
            "secret-two",
            "--force",
            check=True,
            )

        final_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        final_clients = _trojan_clients(final_inbounds)
        assert len(final_clients) == 1
        assert final_clients[0].get("password") == "secret-two"
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_user_remove_is_idempotent(
    server_host, xp2p_server_runner, xp2p_server_run_factory, xp2p_msi_path
):
    _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        link_host = _link_host(server_host)
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62032",
            "--force",
            check=True,
            )
        _ensure_live_inbounds(server_host, xp2p_server_runner, xp2p_server_run_factory)

        _remove_initial_install_client(server_host, xp2p_server_runner)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "bravo",
            "--password",
            "secret",
            check=True,
            )

        xp2p_server_runner(
            "server",
            "user",
            "remove",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--id",
            "bravo",
            check=True,
            )

        after_remove = _read_remote_json(server_host, SERVER_INBOUNDS)
        assert _trojan_clients(after_remove) == []

        xp2p_server_runner(
            "server",
            "user",
            "remove",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--id",
            "bravo",
            check=True,
            )
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_user_add_validates_input(
    server_host, xp2p_server_runner, xp2p_server_run_factory, xp2p_msi_path
):
    _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        link_host = _link_host(server_host)
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62033",
            "--force",
            check=True,
            )
        _ensure_live_inbounds(server_host, xp2p_server_runner, xp2p_server_run_factory)

        _remove_initial_install_client(server_host, xp2p_server_runner)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "charlie",
            check=True,
            )

        current_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        clients = _trojan_clients(current_inbounds)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
        assert _is_unreserved(clients[0].get("password") or "")

        invalid_password = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--id",
            "delta",
            "--password",
            "bad+pass",
            )
        assert invalid_password.rc != 0, "Expected failure when password is invalid"

        missing_id = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            link_host,
            "--password",
            "secret",
            )
        assert missing_id.rc != 0, "Expected failure when identifier is missing"

        current_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        clients = _trojan_clients(current_inbounds)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
def _reset_server_install(server_host, runner, msi_path: str) -> None:
    runner(
        "server",
        "remove",
        "--path",
        str(SERVER_INSTALL_DIR),
        "--ignore-missing",
    )
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[_env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME],
        state_files=SERVER_STATE_FILES,
    )
