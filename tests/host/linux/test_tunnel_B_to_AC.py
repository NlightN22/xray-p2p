from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_A_IP = "10.62.10.11"  # deb-test-a
SERVER_C_IP = "10.62.10.13"  # deb-test-c


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


def _install_server(host, runner, host_ip: str):
    install = runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--host",
        host_ip,
        "--force",
        check=True,
    )
    return helpers.extract_trojan_credential(install.stdout or "")


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_B_to_A_and_C(linux_host_factory, xp2p_linux_versions):
    server_a = linux_host_factory(linux_env.DEFAULT_CLIENT)
    client_b = linux_host_factory(linux_env.DEFAULT_SERVER)
    server_c = linux_host_factory(linux_env.DEFAULT_AUX)
    server_a_runner = _runner(server_a)
    server_c_runner = _runner(server_c)
    client_runner = _runner(client_b)

    def cleanup():
        helpers.cleanup_server_install(server_a, server_a_runner)
        helpers.cleanup_server_install(server_c, server_c_runner)
        helpers.cleanup_client_install(client_b, client_runner)

    cleanup()
    try:
        cred_a = _install_server(server_a, server_a_runner, SERVER_A_IP)
        cred_c = _install_server(server_c, server_c_runner, SERVER_C_IP)

        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            cred_a["link"],
            "--force",
            check=True,
        )
        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            cred_c["link"],
            "--force",
            check=True,
        )

        with linux_env.xp2p_run_session(
            server_a,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ):
            with linux_env.xp2p_run_session(
                server_c,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
                helpers.SERVER_LOG_FILE,
            ):
                with linux_env.xp2p_run_session(
                    client_b,
                    "client",
                    helpers.INSTALL_ROOT.as_posix(),
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    helpers.CLIENT_LOG_FILE,
                ):
                    for target in (SERVER_A_IP, SERVER_C_IP):
                        result = client_runner(
                            "ping",
                            target,
                            "--socks",
                            "--count",
                            "3",
                            check=True,
                        )
                        stdout = (result.stdout or "").lower()
                        assert "0% loss" in stdout, (
                            f"xp2p ping to {target} did not report zero loss:\n"
                            f"{result.stdout}"
                        )
    finally:
        cleanup()
