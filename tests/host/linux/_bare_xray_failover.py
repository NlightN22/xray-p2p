from __future__ import annotations

from . import _bare_xray as bare
from . import _bare_xray_heartbeat as heartbeat
from . import _ha_control_plane_helpers as ha_helpers
from . import _helpers as helpers
from . import _heartbeat_sidecar as heartbeat_sidecar
from . import _runtime_disable as runtime
from . import env as linux_env


AUX_SERVER_IP = "10.62.10.13"


def run(client_host, server_host, aux_host, client_runner, install) -> None:
    aux_runner = runtime.xp2p_runner(aux_host)
    external_tag = helpers.expected_proxy_tag(bare.TLS_NAME)
    full_tag = helpers.expected_proxy_tag(AUX_SERVER_IP)
    try:
        with bare.running(server_host):
            install(client_host, client_runner, server_host, "trojan")
            installed = aux_runner(
                "server",
                "install",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--port",
                "58543",
                "--host",
                AUX_SERVER_IP,
                "--force",
                check=True,
            )
            full_link = helpers.extract_trojan_credential(
                installed.stdout or ""
            )["link"]
            client_runner(
                "client",
                "install",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--link",
                full_link,
                "--force",
                check=True,
            )
            _append_group(client_host, external_tag, full_tag)
            aux_runner("server", "service", "start", check=True)
            runtime.wait_for_service(aux_host, "server", active=True)

            with heartbeat_sidecar.late_sidecar(server_host, "trojan"):
                client_runner("client", "service", "start", check=True)
                bare.wait_for_socks(client_host)
                heartbeat.wait_entry(
                    client_host, bare.TLS_NAME, "healthy", "xp2p-diag"
                )
                heartbeat.wait_entry(
                    client_host, AUX_SERVER_IP, "healthy", "xp2p-heartbeat"
                )
                heartbeat.wait_entry(
                    aux_host,
                    AUX_SERVER_IP,
                    "healthy",
                    "xp2p-heartbeat",
                    path=helpers.SERVER_HEARTBEAT_STATE_FILE,
                )
                ha_helpers.wait_for_group_active(
                    client_runner, external_tag, timeout_seconds=45.0
                )
                _assert_runtime_target(client_host, client_runner, bare.SERVER_IP)

            heartbeat.assert_failure_threshold(
                client_host,
                check="xp2p-diag",
                before_status="healthy",
                threshold_status="unhealthy",
                failure_stage="probe",
            )
            ha_helpers.wait_for_group_active(
                client_runner, full_tag, timeout_seconds=45.0
            )
            _assert_runtime_target(client_host, client_runner, AUX_SERVER_IP)
            full_entry = heartbeat.wait_entry(
                client_host, AUX_SERVER_IP, "healthy", "xp2p-heartbeat"
            )
            assert full_entry.get("failure_stage") in (None, "")

            with heartbeat_sidecar.late_sidecar(server_host, "trojan"):
                heartbeat.wait_entry(
                    client_host, bare.TLS_NAME, "healthy", "xp2p-diag"
                )
                ha_helpers.wait_for_group_active(
                    client_runner, external_tag, timeout_seconds=45.0
                )
                _assert_runtime_target(
                    client_host, client_runner, bare.SERVER_IP
                )
    except Exception:
        bare.failure_dump(client_host, server_host)
        helpers.dump_failure_state(aux_host, "xp2pdiag-failover-aux")
        raise
    finally:
        client_runner("client", "service", "stop")
        aux_runner("server", "service", "stop")
        bare.stop(server_host)
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "remove",
            bare.TLS_NAME,
        )


def _append_group(client_host, external_tag: str, full_tag: str) -> None:
    content = helpers.read_text(client_host, helpers.CLIENT_CONFIG_FILE)
    content += f"""

[[client.endpoint_groups]]
group_id = "{ha_helpers.CLIENT_GROUP_ID}"
tag = "{ha_helpers.CLIENT_GROUP_TAG}"
members = ["{external_tag}", "{full_tag}"]
mode = "automatic"
failure_threshold = 1
success_threshold = 1
cooldown_seconds = 0
minimum_hold_seconds = 0
automatic_failback = true
"""
    helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, content)


def _assert_runtime_target(client_host, runner, expected_host: str) -> None:
    live = helpers.render_xray(client_host, runner, "client", desired=False)
    outbound = next(
        item
        for item in live.get("outbounds") or []
        if item.get("tag") == ha_helpers.CLIENT_GROUP_TAG
    )
    server = ((outbound.get("settings") or {}).get("servers") or [{}])[0]
    assert server.get("address") == expected_host, outbound
