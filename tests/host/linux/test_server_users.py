from __future__ import annotations

import re

import pytest

from tests.host.linux import _helpers as helpers

def _trojan_clients(server_host, xp2p_server_runner) -> list[dict]:
    data = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    inbounds = data.get("inbounds", [])
    for entry in inbounds:
        if entry.get("protocol") == "trojan":
            settings = entry.get("settings", {})
            return settings.get("clients", [])
    pytest.fail("Proxy inbound not found in configuration")


def _remove_default_user(server_host, xp2p_server_runner, host: str):
    clients = _trojan_clients(server_host, xp2p_server_runner)
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
    assert _trojan_clients(server_host, xp2p_server_runner) == []
    return default_client


def _is_unreserved(value: str) -> bool:
    return re.fullmatch(r"[A-Za-z0-9._~-]+", value or "") is not None


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
    try:
        host = "srv-install.xp2p.test"
        _install_server(server_host, xp2p_server_runner, "62040", host)
        default_client = _trojan_clients(server_host, xp2p_server_runner)[0]
        assert default_client["email"].startswith("client-")

        removed = _remove_default_user(server_host, xp2p_server_runner, host)
        assert removed["email"].startswith("client-")
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_user_add_requires_force_for_existing_user(server_host, xp2p_server_runner):
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

        first = _trojan_clients(server_host, xp2p_server_runner)
        assert len(first) == 1 and first[0]["password"] == "secret-one"

        result = xp2p_server_runner(
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
            check=False,
        )
        assert result.rc != 0, "Expected duplicate user add without --force to fail"
        assert "already exists" in (result.stderr or "").lower()
        second = _trojan_clients(server_host, xp2p_server_runner)
        assert len(second) == 1 and second[0]["password"] == "secret-one"

        result = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--force",
            "--id",
            "alpha",
            "--password",
            "secret-two",
            "--host",
            host,
            check=True,
        )
        final = _trojan_clients(server_host, xp2p_server_runner)
        assert len(final) == 1 and final[0]["password"] == "secret-two"
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_user_remove_is_idempotent(server_host, xp2p_server_runner):
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

        assert _trojan_clients(server_host, xp2p_server_runner) == []
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_user_add_validates_password(server_host, xp2p_server_runner):
    try:
        host = "srv-validate.xp2p.test"
        _install_server(server_host, xp2p_server_runner, "62043", host)
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
            "charlie",
            "--host",
            host,
            check=True,
        )
        clients = _trojan_clients(server_host, xp2p_server_runner)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
        assert _is_unreserved(clients[0].get("password") or "")

        invalid_password = xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "delta",
            "--password",
            "bad+pass",
            "--host",
            host,
            check=False,
        )
        assert invalid_password.rc != 0, "Expected invalid password to fail"
        clients = _trojan_clients(server_host, xp2p_server_runner)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
    finally:
        pass
