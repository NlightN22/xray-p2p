from __future__ import annotations

import re

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]


def _runner(host: Host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _read_trojan_users(host: Host) -> dict[str, str]:
    helpers.ensure_service_running(host, "server")
    helpers.wait_for_live_config(host, "server")
    data = helpers.read_live_json(host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
    for inbound in data.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        if inbound.get("protocol") != "trojan":
            continue
        settings = inbound.get("settings") or {}
        clients = settings.get("clients") or []
        users: dict[str, str] = {}
        for entry in clients:
            if not isinstance(entry, dict):
                continue
            user_id = entry.get("email")
            password = entry.get("password")
            if user_id and password:
                users[str(user_id)] = str(password)
        return users
    return {}


def _is_unreserved(value: str) -> bool:
    return re.fullmatch(r"[A-Za-z0-9._~-]+", value or "") is not None


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_user_add_requires_force_for_duplicate(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)

    runner = _runner(openwrt_host)
    try:
        helpers.cleanup_server_install(openwrt_host, runner)

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            "srv-users.local",
            "--port",
            "62090",
            "--force",
            check=True,
        )

        runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha@example.com",
            "--password",
            "alpha-pass",
            "--host",
            "srv-users.local",
            check=True,
        )

        duplicate = runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha@example.com",
            "--password",
            "alpha-pass-2",
            "--host",
            "srv-users.local",
            check=False,
        )
        assert duplicate.rc != 0
        combined = f"{duplicate.stdout}\n{duplicate.stderr}".lower()
        assert "alpha@example.com" in combined and "already exists" in combined

        runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "alpha@example.com",
            "--password",
            "alpha-pass-2",
            "--host",
            "srv-users.local",
            "--force",
            check=True,
        )

        users = _read_trojan_users(openwrt_host)
        assert users.get("alpha@example.com") == "alpha-pass-2"
    finally:
        helpers.cleanup_server_install(openwrt_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_user_add_validates_password(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)

    runner = _runner(openwrt_host)
    try:
        helpers.cleanup_server_install(openwrt_host, runner)

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            "srv-validate.local",
            "--port",
            "62091",
            "--force",
            check=True,
        )

        runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "charlie@example.com",
            "--host",
            "srv-validate.local",
            check=True,
        )
        users = _read_trojan_users(openwrt_host)
        assert users.get("charlie@example.com")
        assert _is_unreserved(users.get("charlie@example.com"))

        invalid_password = runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "delta@example.com",
            "--password",
            "bad+pass",
            "--host",
            "srv-validate.local",
            check=False,
        )
        assert invalid_password.rc != 0, "Expected invalid password to fail"
        users = _read_trojan_users(openwrt_host)
        assert "charlie@example.com" in users
        assert "delta@example.com" not in users
    finally:
        helpers.cleanup_server_install(openwrt_host, runner)
