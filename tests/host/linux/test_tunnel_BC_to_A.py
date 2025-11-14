from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_IP = "10.62.10.11"  # deb-test-a


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = linux_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _extract_link(output: str) -> str:
    for raw in (output or "").splitlines():
        stripped = raw.strip()
        if stripped.startswith("trojan://"):
            return stripped
    pytest.fail(f"xp2p server user add did not emit trojan link.\nSTDOUT:\n{output}")


def _install_client(host, runner, link: str):
    helpers.cleanup_client_install(host, runner)
    runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--link",
        link,
        "--force",
        check=True,
    )


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_BC_to_A(linux_host_factory, xp2p_linux_versions):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)
    client_b = linux_host_factory(linux_env.DEFAULT_SERVER)
    client_c = linux_host_factory(linux_env.DEFAULT_AUX)

    server_runner = _runner(server_host)
    client_b_runner = _runner(client_b)
    client_c_runner = _runner(client_c)

    helpers.cleanup_server_install(server_host, server_runner)
    helpers.cleanup_client_install(client_b, client_b_runner)
    helpers.cleanup_client_install(client_c, client_c_runner)

    try:
        server_install = server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--port",
            "62070",
            "--force",
            check=True,
        )
        default_cred = helpers.extract_trojan_credential(server_install.stdout or "")

        user_add = server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            "client-two@example.com",
            "--password",
            "client-two-pass",
            "--host",
            SERVER_IP,
            check=True,
        )
        second_link = _extract_link(user_add.stdout or "")

        _install_client(client_b, client_b_runner, default_cred["link"])
        _install_client(client_c, client_c_runner, second_link)

        with linux_env.xp2p_run_session(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ), linux_env.xp2p_run_session(
            client_b,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE,
        ), linux_env.xp2p_run_session(
            client_c,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE,
        ):
            for runner, origin in ((client_b_runner, "client-b"), (client_c_runner, "client-c")):
                result = runner(
                    "ping",
                    SERVER_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                stdout = (result.stdout or "").lower()
                assert "0% loss" in stdout, (
                    f"xp2p ping from {origin} did not report zero loss:\n"
                    f"{result.stdout}"
                )
    finally:
        helpers.cleanup_client_install(client_b, client_b_runner)
        helpers.cleanup_client_install(client_c, client_c_runner)
        helpers.cleanup_server_install(server_host, server_runner)
