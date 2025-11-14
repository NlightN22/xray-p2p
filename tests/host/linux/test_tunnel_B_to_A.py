from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)


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


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_B_to_A(linux_host_factory, xp2p_linux_versions):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)  # Host A
    client_host = linux_host_factory(linux_env.DEFAULT_SERVER)  # Host B
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup():
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)

    cleanup()
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
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"

        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            credential["link"],
            "--force",
            check=True,
        )

        with linux_env.xp2p_run_session(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ):
            with linux_env.xp2p_run_session(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
            ):
                ping_result = client_runner(
                    "ping",
                    SERVER_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                stdout = (ping_result.stdout or "").lower()
                assert "0% loss" in stdout, (
                    "xp2p ping did not report full delivery:\n"
                    f"{ping_result.stdout}"
                )
    finally:
        cleanup()
