from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _server_user_helpers as user_helpers


@pytest.mark.host
@pytest.mark.linux
def test_server_install_provisions_default_user(server_host, xp2p_server_runner):
    try:
        host = "srv-install.xp2p.test"
        install = user_helpers.install_server(server_host, xp2p_server_runner, "62040", host)
        credential = helpers.parse_json_credential(install.stdout or "")
        default_client = user_helpers.trojan_clients(server_host, xp2p_server_runner)[0]
        assert user_helpers.is_generated_user_id(credential["user"])
        assert user_helpers.is_uuid(credential["password"])
        user_helpers.assert_trojan_link_matches_credential(
            credential["link"],
            credential["user"],
            credential["password"],
            host,
        )
        assert default_client == {"email": credential["user"], "password": credential["password"]}

        removed = user_helpers.remove_default_user(server_host, xp2p_server_runner, host)
        assert removed == default_client
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_user_add_generates_compatible_password(server_host, xp2p_server_runner):
    try:
        host = "srv-validate.xp2p.test"
        user_helpers.install_server(server_host, xp2p_server_runner, "62043", host)
        user_helpers.remove_default_user(server_host, xp2p_server_runner, host)

        xp2p_server_runner(
            "server",
            "user",
            "add", "--json",
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
        clients = user_helpers.trojan_clients(server_host, xp2p_server_runner)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
        assert user_helpers.is_unreserved(clients[0].get("password") or "")
        assert user_helpers.is_uuid(clients[0].get("password") or "")

        invalid_password = xp2p_server_runner(
            "server",
            "user",
            "add", "--json",
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
        clients = user_helpers.trojan_clients(server_host, xp2p_server_runner)
        assert len(clients) == 1
        assert clients[0].get("email") == "charlie"
    finally:
        pass
