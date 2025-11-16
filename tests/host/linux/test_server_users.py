from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers

SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"


def _cleanup(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)


def _trojan_clients(server_host) -> list[dict]:
    data = helpers.read_json(server_host, SERVER_INBOUNDS)
    inbounds = data.get("inbounds", [])
    for entry in inbounds:
        if entry.get("protocol") == "trojan":
            settings = entry.get("settings", {})
            return settings.get("clients", [])
    pytest.fail("Trojan inbound not found in configuration")


def _remove_default_user(server_host, xp2p_server_runner, host: str):
    clients = _trojan_clients(server_host)
    assert clients, "Expected default client from server install"
    default_client = clients[0]
    xp2p_server_runner(
        "server",
        "user",
        "remove",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        default_client["email"],
        "--host",
        host,
        check=True,
    )
    assert _trojan_clients(server_host) == []
    return default_client


def _install_server(server_host, xp2p_server_runner, port: str, host: str):
    xp2p_server_runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        port,
        "--host",
        host,
        "--force",
        check=True,
    )


@pytest.mark.host
@pytest.mark.linux
def test_server_install_provisions_default_user(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    try:
        host = "srv-install.xp2p.test"
        _install_server(server_host, xp2p_server_runner, "62040", host)
        default_client = _trojan_clients(server_host)[0]
        assert default_client["email"].startswith("client-")

        removed = _remove_default_user(server_host, xp2p_server_runner, host)
        assert removed["email"].startswith("client-")
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_user_add_is_idempotent(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    try:
        host = "srv-add.xp2p.test"
        _install_server(server_host, xp2p_server_runner, "62041", host)
        _remove_default_user(server_host, xp2p_server_runner, host)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            "--host",
            host,
            check=True,
        )

        first = _trojan_clients(server_host)
        assert len(first) == 1 and first[0]["password"] == "secret-one"

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha",
            "--password",
            "secret-one",
            "--host",
            host,
            check=True,
        )
        second = _trojan_clients(server_host)
        assert len(second) == 1 and second[0]["password"] == "secret-one"

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha",
            "--password",
            "secret-two",
            "--host",
            host,
            check=True,
        )
        final = _trojan_clients(server_host)
        assert len(final) == 1 and final[0]["password"] == "secret-two"
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_user_remove_is_idempotent(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    try:
        host = "srv-remove.xp2p.test"
        _install_server(server_host, xp2p_server_runner, "62042", host)
        _remove_default_user(server_host, xp2p_server_runner, host)

        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "bravo",
            "--password",
            "secret",
            "--host",
            host,
            check=True,
        )

        xp2p_server_runner(
            "server",
            "user",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "bravo",
            "--host",
            host,
            check=True,
        )

        xp2p_server_runner(
            "server",
            "user",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "bravo",
            "--host",
            host,
            check=True,
        )

        assert _trojan_clients(server_host) == []
    finally:
        _cleanup(server_host, xp2p_server_runner)
