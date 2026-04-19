from __future__ import annotations

from tests.host.linux import _helpers as helpers
from tests.host.linux import parsers
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.tunnel import common as tunnel_common


def assert_reverse_redirect_via_server_portal(env: dict) -> None:
    server_runner = env["server_runner"]
    server_install_path = env["server_install_path"]
    reverse_tag = env["reverse_tag"]
    client_host = env["client_host"]
    server_host = env["server_host"]
    reverse_channels = helpers.read_pending_server_config(server_host).get("reverse_channels") or {}
    if reverse_tag not in reverse_channels:
        server_runner(
            "server",
            "user",
            "add",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            env["client_user"],
            "--password",
            env["client_password"],
            "--host",
            fixture.SERVER_IP,
            "--force",
            check=True,
        )

    alias_cidr = f"{fixture.CLIENT_REVERSE_TEST_IP}/32"
    with fixture.ip_alias(client_host, alias_cidr):
        server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            alias_cidr,
            "--tag",
            reverse_tag,
            check=True,
        )
        forward_added = False
        try:
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
            list_entries = parsers.parse_redirect_output(list_output)
            assert parsers.has_redirect_entry(list_entries, cidr=alias_cidr, tag=reverse_tag), (
                f"Server redirect list missing {alias_cidr} for tag {reverse_tag}"
            )

            server_state = helpers.read_pending_server_config(server_host)
            server_routing = helpers.render_xray(server_host, server_runner, "server", desired=True)
            helpers.assert_server_redirect_state(server_state, alias_cidr, reverse_tag)
            helpers.assert_server_redirect_rule(server_routing, alias_cidr, reverse_tag)

            fixture.server_forward_cmd(
                env,
                "remove",
                "--listen-port",
                str(fixture.SERVER_FORWARD_PORT),
                check=False,
            )
            server_runner(
                "server",
                "forward",
                "add",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--target",
                f"{fixture.CLIENT_REVERSE_TEST_IP}:{fixture.CLIENT_DIAGNOSTICS_PORT}",
                "--listen",
                "127.0.0.1",
                "--listen-port",
                str(fixture.SERVER_FORWARD_PORT),
                "--proto",
                "tcp",
                check=True,
            )
            forward_added = True

            server_state = helpers.read_pending_server_config(server_host)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], fixture.CLIENT_REVERSE_TEST_IP, fixture.CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == fixture.SERVER_FORWARD_PORT

            with fixture.active_tunnel_sessions(env):
                ping_result = server_runner(
                    "ping",
                    "127.0.0.1",
                    "--port",
                    str(fixture.SERVER_FORWARD_PORT),
                    "--count",
                    "3",
                    check=True,
                )
                tunnel_common.assert_zero_loss(ping_result, f"via server forward targeting {fixture.CLIENT_REVERSE_TEST_IP}")
        finally:
            if forward_added:
                fixture.server_forward_cmd(
                    env,
                    "remove",
                    "--listen-port",
                    str(fixture.SERVER_FORWARD_PORT),
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
                "--cidr",
                alias_cidr,
                "--tag",
                reverse_tag,
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
            final_entries = parsers.parse_redirect_output(final_list)
            assert not parsers.has_redirect_entry(final_entries, cidr=alias_cidr, tag=reverse_tag)

