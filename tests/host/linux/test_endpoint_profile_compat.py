from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime

pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.serial]

HOST = "legacy-profile.example.com"
PORT = "62311"
USER_LABEL = "legacy@example.com"
LEGACY_CREDENTIAL = "legacy-credential"
UPDATED_CREDENTIAL = "550e8400-e29b-41d4-a716-446655440000"


def test_server_user_update_normalizes_legacy_trojan_state(server_host):
    runner = runtime.xp2p_runner(server_host)
    runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--host",
        HOST,
        "--port",
        PORT,
        "--force",
        check=True,
    )
    helpers.write_text(
        server_host,
        helpers.SERVER_CONFIG_FILE,
        "\n".join(
            [
                "[server]",
                f'trojan_port = "{PORT}"',
                "trojan_users = [{ email = \"legacy@example.com\", password = \"legacy-credential\" }]",
                "",
            ]
        ),
    )

    runner(
        "server",
        "user",
        "update",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        USER_LABEL,
        "--password",
        UPDATED_CREDENTIAL,
        check=True,
    )

    desired = helpers.read_toml(server_host, helpers.SERVER_CONFIG_FILE).get("server") or {}
    assert "trojan_users" not in desired
    users = desired.get("users") or []
    assert users == [{"user_label": USER_LABEL, "credential": UPDATED_CREDENTIAL}]
