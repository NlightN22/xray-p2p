import pytest

from .test_server import (
    SERVER_CONFIG_DIR_NAME,
    SERVER_INBOUNDS,
    SERVER_INSTALL_DIR,
    _cleanup_server_install,
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


@pytest.mark.host
def test_server_user_add_and_idempotent(server_host, xp2p_server_runner):
    _cleanup_server_install(xp2p_server_runner)
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
        _cleanup_server_install(xp2p_server_runner)


@pytest.mark.host
def test_server_user_remove_is_idempotent(server_host, xp2p_server_runner):
    _cleanup_server_install(xp2p_server_runner)
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
        _cleanup_server_install(xp2p_server_runner)


@pytest.mark.host
def test_server_user_add_validates_input(server_host, xp2p_server_runner):
    _cleanup_server_install(xp2p_server_runner)
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
        _cleanup_server_install(xp2p_server_runner)
