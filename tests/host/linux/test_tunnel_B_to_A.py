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

    server_install_path = helpers.INSTALL_ROOT.as_posix()

    def cleanup():
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)

    cleanup()
    try:
        server_install = server_runner(
            "server",
            "install",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)

        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_reverse_state(
            server_state,
            reverse_tag,
            user=credential["user"],
            host=SERVER_IP,
        )
        helpers.assert_server_reverse_routing(server_routing, reverse_tag, user=credential["user"])

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
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
        )
        recorded_tags = {tag for tag in client_state.get("reverse", {}).keys()}
        assert recorded_tags == {reverse_tag}, f"Unexpected reverse entries recorded: {recorded_tags}"

        redirect_domain = f"full:{reverse_tag}"
        redirect_added = False

        try:
            server_runner(
                "server",
                "redirect",
                "add",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--domain",
                redirect_domain,
                "--host",
                SERVER_IP,
                check=True,
            )
            redirect_added = True
            list_output = server_runner(
                "server",
                "redirect",
                "list",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=True,
            ).stdout or ""
            assert redirect_domain in list_output.lower(), f"Server redirect list missing {redirect_domain}"
            server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
            server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
            helpers.assert_server_redirect_state(server_state, redirect_domain, reverse_tag)
            helpers.assert_server_redirect_rule(server_routing, redirect_domain, reverse_tag)

            with linux_env.xp2p_run_session(
                server_host,
                "server",
                server_install_path,
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
            if redirect_added:
                server_runner(
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    server_install_path,
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    redirect_domain,
                    "--host",
                    SERVER_IP,
                    check=True,
                )
                final_list = server_runner(
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    server_install_path,
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert "no server redirect rules configured" in final_list.lower()
    finally:
        cleanup()
