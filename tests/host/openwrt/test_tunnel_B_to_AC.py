from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

SERVER_A_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_C_MACHINE = openwrt_env.OPENWRT_MACHINES[2]
SERVER_A_IP = "10.63.30.11"
SERVER_C_IP = "10.63.30.13"
CLIENT_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_ROUTING = helpers.CLIENT_CONFIG_DIR / "routing.json"


@dataclass
class RedirectSpec:
    runner: Callable[..., Any]
    host: Any
    ip: str
    domain: str
    reverse_tag: str


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
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
def test_tunnel_B_to_A_and_C(openwrt_host_factory, xp2p_openwrt_ipk):
    server_a = openwrt_host_factory(SERVER_A_MACHINE)
    client_b = openwrt_host_factory(CLIENT_MACHINE)
    server_c = openwrt_host_factory(SERVER_C_MACHINE)

    for machine, host in (
        (SERVER_A_MACHINE, server_a),
        (SERVER_C_MACHINE, server_c),
        (CLIENT_MACHINE, client_b),
    ):
        openwrt_env.sync_build_output(machine)
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    server_a_runner = _runner(server_a)
    server_c_runner = _runner(server_c)
    client_runner = _runner(client_b)
    client_primary_ip = helpers.detect_primary_ipv4(client_b)

    def cleanup():
        helpers.cleanup_server_install(server_a, server_a_runner)
        helpers.cleanup_server_install(server_c, server_c_runner)
        helpers.cleanup_client_install(client_b, client_runner)
        for host in (server_a, server_c, client_b):
            helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)

    cleanup()
    try:
        cred_a = _install_server(server_a, server_a_runner, SERVER_A_IP)
        cred_c = _install_server(server_c, server_c_runner, SERVER_C_IP)

        server_entries = [
            {
                "host": server_a,
                "runner": server_a_runner,
                "ip": SERVER_A_IP,
                "credential": cred_a,
                "reverse_tag": helpers.expected_reverse_tag(cred_a["user"], SERVER_A_IP),
            },
            {
                "host": server_c,
                "runner": server_c_runner,
                "ip": SERVER_C_IP,
                "credential": cred_c,
                "reverse_tag": helpers.expected_reverse_tag(cred_c["user"], SERVER_C_IP),
            },
        ]
        for entry in server_entries:
            server_state = helpers.read_first_existing_json(entry["host"], helpers.SERVER_STATE_FILES)
            server_routing = helpers.read_json(entry["host"], helpers.SERVER_CONFIG_DIR / "routing.json")
            helpers.assert_server_reverse_state(
                server_state,
                entry["reverse_tag"],
                user=entry["credential"]["user"],
                host=entry["ip"],
            )
            helpers.assert_server_reverse_routing(
                server_routing, entry["reverse_tag"], user=entry["credential"]["user"]
            )
            helpers.assert_reverse_cli_output(
                entry["runner"],
                "server",
                helpers.INSTALL_ROOT,
                helpers.SERVER_CONFIG_DIR_NAME,
                entry["reverse_tag"],
            )

        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            cred_a["link"],
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
            check=True,
        )

        client_state = helpers.read_first_existing_json(client_b, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_b, CLIENT_ROUTING)

        outbounds = helpers.read_json(client_b, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            outbounds,
            SERVER_A_IP,
            cred_a["password"],
            cred_a["user"],
            SERVER_A_IP,
            allow_insecure=True,
        )
        helpers.assert_outbound(
            outbounds,
            SERVER_C_IP,
            cred_c["password"],
            cred_c["user"],
            SERVER_C_IP,
            allow_insecure=True,
        )

        routing = client_routing
        helpers.assert_routing_rule(routing, SERVER_A_IP)
        helpers.assert_routing_rule(routing, SERVER_C_IP)

        state = client_state
        recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert recorded_hosts == {SERVER_A_IP, SERVER_C_IP}
        reverse_entries = state.get("reverse", {}) or {}
        expected_reverse_tags = {entry["reverse_tag"] for entry in server_entries}
        assert set(reverse_entries.keys()) == expected_reverse_tags, (
            "Client reverse entries do not match installed servers"
        )

        for entry in server_entries:
            endpoint_tag = helpers.expected_proxy_tag(entry["ip"])
            helpers.assert_client_reverse_artifacts(client_routing, entry["reverse_tag"], endpoint_tag)
            helpers.assert_client_reverse_state(
                client_state,
                entry["reverse_tag"],
                endpoint_tag=endpoint_tag,
                user=entry["credential"]["user"],
                host=entry["ip"],
            )
            helpers.assert_reverse_cli_output(
                client_runner,
                "client",
                helpers.INSTALL_ROOT,
                helpers.CLIENT_CONFIG_DIR_NAME,
                entry["reverse_tag"],
            )

        redirect_specs: list[RedirectSpec] = []
        try:
            for entry in server_entries:
                redirect_domain = f"full:{entry['reverse_tag']}"
                entry["runner"](
                    "server",
                    "redirect",
                    "add",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    redirect_domain,
                    "--host",
                    entry["ip"],
                    check=True,
                )
                redirect_specs.append(
                    RedirectSpec(
                        runner=entry["runner"],
                        host=entry["host"],
                        ip=entry["ip"],
                        domain=redirect_domain,
                        reverse_tag=entry["reverse_tag"],
                    )
                )
                list_output = entry["runner"](
                    "server",
                    "redirect",
                    "list",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    check=True,
                ).stdout or ""
                assert redirect_domain in list_output.lower(), (
                    f"Server redirect list missing {redirect_domain} for {entry['ip']}"
                )
                server_state = helpers.read_first_existing_json(entry["host"], helpers.SERVER_STATE_FILES)
                server_routing = helpers.read_json(entry["host"], helpers.SERVER_CONFIG_DIR / "routing.json")
                helpers.assert_server_redirect_state(server_state, redirect_domain, entry["reverse_tag"])
                helpers.assert_server_redirect_rule(server_routing, redirect_domain, entry["reverse_tag"])

            with openwrt_env.xp2p_run_session(
                server_a,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
                helpers.SERVER_LOG_FILE,
            ), openwrt_env.xp2p_run_session(
                server_c,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
                helpers.SERVER_LOG_FILE,
            ), openwrt_env.xp2p_run_session(
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
                for entry in server_entries:
                    heartbeat_state = helpers.wait_for_heartbeat_state(entry["host"])
                    helpers.assert_heartbeat_entry(
                        heartbeat_state,
                        helpers.expected_proxy_tag(entry["ip"]),
                        host=entry["ip"],
                        user=entry["credential"]["user"],
                        client_ip=client_primary_ip,
                    )
        finally:
            for spec in redirect_specs:
                spec.runner(
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    spec.domain,
                    "--host",
                    spec.ip,
                    check=True,
                )
                final_list = spec.runner(
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
        cleanup()
