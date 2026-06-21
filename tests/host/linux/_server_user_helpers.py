from __future__ import annotations

import re
from urllib.parse import unquote, urlparse

import pytest

from tests.host.linux import _helpers as helpers


def trojan_clients(server_host, xp2p_server_runner) -> list[dict]:
    data = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    inbounds = data.get("inbounds", [])
    for entry in inbounds:
        if entry.get("protocol") == "trojan":
            settings = entry.get("settings", {})
            return settings.get("clients", [])
    pytest.fail("Proxy inbound not found in configuration")


def remove_user(xp2p_server_runner, user_id: str, host: str) -> None:
    xp2p_server_runner(
        "server",
        "user",
        "remove",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        user_id,
        "--host",
        host,
        check=True,
    )


def remove_default_user(server_host, xp2p_server_runner, host: str) -> dict:
    clients = trojan_clients(server_host, xp2p_server_runner)
    assert clients, "Expected default client from server install"
    default_client = clients[0]
    remove_user(xp2p_server_runner, default_client["email"], host)
    assert trojan_clients(server_host, xp2p_server_runner) == []
    return default_client


def install_server(server_host, xp2p_server_runner, port: str, host: str):
    return xp2p_server_runner(
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


def is_unreserved(value: str) -> bool:
    return re.fullmatch(r"[A-Za-z0-9._~-]+", value or "") is not None


def is_generated_user_id(value: str) -> bool:
    return re.fullmatch(r"client-[a-z2-7]{8}@xp2p\.local", value or "") is not None


def is_uuid(value: str) -> bool:
    return (
        re.fullmatch(
            r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}",
            value or "",
        )
        is not None
    )


def assert_trojan_link_matches_credential(link: str, user: str, password: str, host: str) -> None:
    parsed = urlparse(link)
    assert parsed.scheme == "trojan"
    assert parsed.hostname == host
    assert parsed.username == password
    assert unquote(unquote(parsed.fragment)) == user
