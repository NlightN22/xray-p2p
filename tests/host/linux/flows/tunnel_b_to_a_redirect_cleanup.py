from __future__ import annotations

from tests.host.linux import _helpers as helpers
from tests.host.linux import parsers
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture


def assert_server_redirect_cleanup_on_user_remove(env: dict) -> None:
    server_runner = env["server_runner"]
    server_install_path = env["server_install_path"]
    primary_tag = env["reverse_tag"]
    cleanup_user = "cleanup-redirect-user"
    cleanup_password = "cleanup-redirect-secret"
    cleanup_tag = helpers.expected_reverse_tag(cleanup_user, fixture.SERVER_IP)
    cleanup_cidr = f"{fixture.CLIENT_REVERSE_TEST_IP}/32"
    recovery_user = "cleanup-recovery-user"
    recovery_password = "cleanup-recovery-secret"
    recovery_tag = helpers.expected_reverse_tag(recovery_user, fixture.SERVER_IP)
    recovery_cidr = "10.0.102.60/32"

    try:
        _server_user_add(server_runner, server_install_path, cleanup_user, cleanup_password)
        _server_redirect_add(server_runner, server_install_path, cleanup_cidr, primary_tag)
        _server_redirect_add(server_runner, server_install_path, cleanup_cidr, cleanup_tag)

        entries = _server_redirect_entries(server_runner, server_install_path)
        assert parsers.has_redirect_entry(entries, cidr=cleanup_cidr, tag=primary_tag)
        assert parsers.has_redirect_entry(entries, cidr=cleanup_cidr, tag=cleanup_tag)

        _server_user_remove(server_runner, server_install_path, cleanup_user, check=True)

        entries = _server_redirect_entries(server_runner, server_install_path)
        assert parsers.has_redirect_entry(entries, cidr=cleanup_cidr, tag=primary_tag)
        assert not parsers.has_redirect_entry(entries, cidr=cleanup_cidr, tag=cleanup_tag)
        state = helpers.read_pending_server_config(env["server_host"])
        assert cleanup_tag not in (state.get("reverse_channels") or {})

        _server_user_add(server_runner, server_install_path, recovery_user, recovery_password)
        _server_redirect_add(server_runner, server_install_path, recovery_cidr, recovery_tag)
        entries = _server_redirect_entries(server_runner, server_install_path)
        assert parsers.has_redirect_entry(entries, cidr=recovery_cidr, tag=recovery_tag)

        server_runner(
            "server",
            "redirect",
            "remove",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--tag",
            recovery_tag,
            check=True,
        )
        entries = _server_redirect_entries(server_runner, server_install_path)
        assert not parsers.has_redirect_entry(entries, cidr=recovery_cidr, tag=recovery_tag)
    finally:
        server_runner(
            "server",
            "redirect",
            "remove",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            cleanup_cidr,
            "--tag",
            primary_tag,
            check=False,
        )
        server_runner(
            "server",
            "redirect",
            "remove",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--tag",
            cleanup_tag,
            check=False,
        )
        server_runner(
            "server",
            "redirect",
            "remove",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--tag",
            recovery_tag,
            check=False,
        )
        _server_user_remove(server_runner, server_install_path, cleanup_user, check=False)
        _server_user_remove(server_runner, server_install_path, recovery_user, check=False)


def _server_user_add(runner, install_path: str, user: str, password: str) -> None:
    runner(
        "server",
        "user",
        "add",
        "--path",
        install_path,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        user,
        "--password",
        password,
        "--host",
        fixture.SERVER_IP,
        "--force",
        check=True,
    )


def _server_user_remove(runner, install_path: str, user: str, *, check: bool) -> None:
    runner(
        "server",
        "user",
        "remove",
        "--path",
        install_path,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        user,
        check=check,
    )


def _server_redirect_add(runner, install_path: str, cidr: str, tag: str) -> None:
    runner(
        "server",
        "redirect",
        "add",
        "--path",
        install_path,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--cidr",
        cidr,
        "--tag",
        tag,
        check=True,
    )


def _server_redirect_entries(runner, install_path: str) -> list[dict[str, str]]:
    output = (
        runner(
            "server",
            "redirect",
            "list",
            "--path",
            install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            check=True,
        ).stdout
        or ""
    )
    return parsers.parse_redirect_output(output)
