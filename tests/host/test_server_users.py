import pytest

from tests.host import _env

from .test_server_install import (
    SERVER_CONFIG_DIR_NAME,
    SERVER_INBOUNDS,
    SERVER_INSTALL_DIR,
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


@pytest.mark.host
def test_server_install_creates_and_allows_removing_default_user(
    server_host, xp2p_server_runner, xp2p_msi_path
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

        default_client = _initial_install_client(server_host)
        assert default_client["email"].startswith("client-")

        _remove_initial_install_client(server_host, xp2p_server_runner)
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
def test_server_user_add_and_idempotent(server_host, xp2p_server_runner, xp2p_msi_path):
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
            "62031",
            "--force",
            check=True,
        )

        _remove_initial_install_client(server_host, xp2p_server_runner)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            check=True,
        )

        first_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        first_clients = _trojan_clients(first_inbounds)
        assert len(first_clients) == 1
        assert first_clients[0].get("email") == "alpha"
        assert first_clients[0].get("password") == "secret-one"

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            check=True,
        )

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
            "--id",
            "alpha",
            "--password",
            "secret-two",
            check=True,
        )

        final_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        final_clients = _trojan_clients(final_inbounds)
        assert len(final_clients) == 1
        assert final_clients[0].get("password") == "secret-two"
    finally:
        _reset_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
def test_server_user_remove_is_idempotent(server_host, xp2p_server_runner, xp2p_msi_path):
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
            "62032",
            "--force",
            check=True,
        )

        _remove_initial_install_client(server_host, xp2p_server_runner)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
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
def test_server_user_add_validates_input(server_host, xp2p_server_runner, xp2p_msi_path):
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
            "62033",
            "--force",
            check=True,
        )

        _remove_initial_install_client(server_host, xp2p_server_runner)

        missing_password = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--id",
            "charlie",
        )
        assert missing_password.rc != 0, "Expected failure when password is missing"

        missing_id = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--password",
            "secret",
        )
        assert missing_id.rc != 0, "Expected failure when identifier is missing"

        current_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        assert _trojan_clients(current_inbounds) == []
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
    _env.install_xp2p_from_msi(server_host, msi_path)
