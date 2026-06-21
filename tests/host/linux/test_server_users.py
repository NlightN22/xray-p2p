from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _server_user_helpers as user_helpers


@pytest.mark.host
@pytest.mark.linux
def test_server_user_add_requires_force_for_existing_user(server_host, xp2p_server_runner):
    try:
        host = "srv-add.xp2p.test"
        user_helpers.install_server(server_host, xp2p_server_runner, "62041", host)
        user_helpers.remove_default_user(server_host, xp2p_server_runner, host)

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

        first = user_helpers.trojan_clients(server_host, xp2p_server_runner)
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
        second = user_helpers.trojan_clients(server_host, xp2p_server_runner)
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
            "bravo",
            "--password",
            "secret-bravo",
            "--host",
            host,
            check=True,
        )
        duplicate_update = xp2p_server_runner(
            "server",
            "user",
            "update",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--new-id",
            "bravo",
            "alpha",
            check=False,
        )
        assert duplicate_update.rc != 0, "Expected user update to reject duplicate target id"
        assert "already exists" in ((duplicate_update.stdout or "") + (duplicate_update.stderr or "")).lower()
        after_duplicate_update = sorted(
            user_helpers.trojan_clients(server_host, xp2p_server_runner),
            key=lambda item: item["email"],
        )
        assert after_duplicate_update == [
            {"email": "alpha", "password": "secret-one"},
            {"email": "bravo", "password": "secret-bravo"},
        ]

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
        final = sorted(user_helpers.trojan_clients(server_host, xp2p_server_runner), key=lambda item: item["email"])
        assert final == [
            {"email": "alpha", "password": "secret-two"},
            {"email": "bravo", "password": "secret-bravo"},
        ]
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_user_remove_is_idempotent(server_host, xp2p_server_runner):
    try:
        host = "srv-remove.xp2p.test"
        apply_request = helpers.STATE_ROOT / "apply.request"
        user_helpers.install_server(server_host, xp2p_server_runner, "62042", host)
        user_helpers.remove_default_user(server_host, xp2p_server_runner, host)

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

        helpers.remove_path(server_host, apply_request)
        user_helpers.remove_user(xp2p_server_runner, "bravo", host)
        assert not helpers.path_exists(server_host, apply_request), "stopped-service user removal should not request apply"

        helpers.remove_path(server_host, apply_request)
        user_helpers.remove_user(xp2p_server_runner, "bravo", host)
        assert not helpers.path_exists(server_host, apply_request), "no-op user removal should not request apply"

        assert user_helpers.trojan_clients(server_host, xp2p_server_runner) == []
    finally:
        pass
