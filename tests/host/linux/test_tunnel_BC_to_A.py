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
        reverse_default = helpers.expected_reverse_tag(default_cred["user"], SERVER_IP)

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
        reverse_second = helpers.expected_reverse_tag("client-two@example.com", SERVER_IP)

        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        for reverse_tag, user in (
            (reverse_default, default_cred["user"]),
            (reverse_second, "client-two@example.com"),
        ):
            helpers.assert_server_reverse_state(
                server_state,
                reverse_tag,
                user=user,
                host=SERVER_IP,
            )
            helpers.assert_server_reverse_routing(server_routing, reverse_tag, user=user)
        recorded_server_tags = set((server_state.get("reverse_channels") or {}).keys())
        assert recorded_server_tags == {reverse_default, reverse_second}

        _install_client(client_b, client_b_runner, default_cred["link"])
        _install_client(client_c, client_c_runner, second_link)

        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        client_b_state = helpers.read_first_existing_json(client_b, helpers.CLIENT_STATE_FILES)
        client_b_routing = helpers.read_json(client_b, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_client_reverse_artifacts(client_b_routing, reverse_default, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_b_state,
            reverse_default,
            endpoint_tag=endpoint_tag,
            user=default_cred["user"],
            host=SERVER_IP,
        )
        assert set((client_b_state.get("reverse") or {}).keys()) == {reverse_default}

        client_c_state = helpers.read_first_existing_json(client_c, helpers.CLIENT_STATE_FILES)
        client_c_routing = helpers.read_json(client_c, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_client_reverse_artifacts(client_c_routing, reverse_second, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_c_state,
            reverse_second,
            endpoint_tag=endpoint_tag,
            user="client-two@example.com",
            host=SERVER_IP,
        )
        assert set((client_c_state.get("reverse") or {}).keys()) == {reverse_second}

        redirect_domains: list[dict[str, str]] = []
        try:
            for reverse_tag in (reverse_default, reverse_second):
                domain = f"full:{reverse_tag}"
                server_runner(
                    "server",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    domain,
                    "--tag",
                    reverse_tag,
                    check=True,
                )
                redirect_domains.append({"domain": domain, "tag": reverse_tag})
                list_output = server_runner(
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert domain in list_output.lower(), f"Server redirect list missing {domain}"
                server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
                server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
                helpers.assert_server_redirect_state(server_state, domain, reverse_tag)
                helpers.assert_server_redirect_rule(server_routing, domain, reverse_tag)

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
            while redirect_domains:
                entry = redirect_domains.pop()
                domain = entry["domain"]
                tag = entry["tag"]
                list_output = server_runner(
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                ).stdout or ""
                listed = (list_output or "").lower()
                if domain not in listed:
                    continue
                removal = server_runner(
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    domain,
                    "--tag",
                    tag,
                    check=False,
                )
                stderr = (removal.stderr or "").lower()
                if removal.rc != 0 and "not found" not in stderr:
                    pytest.fail(
                        f"Failed to remove redirect {domain}:\nSTDOUT:\n{removal.stdout}\nSTDERR:\n{removal.stderr}"
                    )
            final_list = server_runner(
                "server",
                "redirect",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=True,
            ).stdout or ""
            assert "no server redirect rules configured" in final_list.lower()
    finally:
        helpers.cleanup_client_install(client_b, client_b_runner)
        helpers.cleanup_client_install(client_c, client_c_runner)
        helpers.cleanup_server_install(server_host, server_runner)
