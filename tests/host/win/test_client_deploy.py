from __future__ import annotations

import time
from pathlib import Path

import pytest

from tests.host.win import env as win_env
from tests.host.win.assertions import deploy as deploy_assert
from tests.host.win.diagnostics import remote_files
from tests.host.win.flows import apply as apply_flow
from tests.host.win.flows import deploy as deploy_flow


@pytest.mark.host
@pytest.mark.win
def test_windows_client_deploy_end_to_end(
    client_host,
    server_host,
    client_host_ipv4,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
    xp2p_msi_path,
):
    test_start = time.perf_counter()
    with deploy_flow.timed("cleanup xp2p processes (client)"):
        deploy_flow.stop_xp2p_processes(client_host)
    with deploy_flow.timed("cleanup xp2p processes (server)"):
        deploy_flow.stop_xp2p_processes(server_host)
    with deploy_flow.timed("cleanup client socks listeners"):
        deploy_flow.stop_listening_ports(client_host, [51080, 51180])
    with deploy_flow.timed("cleanup server socks listeners"):
        deploy_flow.stop_listening_ports(server_host, [51080, 51180])
    with deploy_flow.timed("xp2p client remove"):
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
    with deploy_flow.timed("remove client config/state"):
        deploy_flow.remove_paths(client_host, [deploy_assert.CLIENT_CONFIG_DIR, *deploy_assert.CLIENT_STATE_FILES])
    with deploy_flow.timed("remove server config/state"):
        deploy_flow.remove_paths(server_host, [deploy_assert.SERVER_CONFIG_DIR, *deploy_assert.SERVER_STATE_FILES])

    with deploy_flow.timed("remove heartbeat state"):
        for host in (client_host, server_host):
            deploy_flow.remove_paths(host, deploy_assert.HEARTBEAT_STATE_FILES)
    with deploy_flow.timed("remove deploy logs (client)"):
        deploy_flow.remove_paths(
            client_host,
            [
                deploy_flow.CLIENT_DEPLOY_STDOUT,
                Path(str(deploy_flow.CLIENT_DEPLOY_STDOUT) + ".err"),
            ],
        )
    with deploy_flow.timed("remove deploy logs (server)"):
        deploy_flow.remove_paths(
            server_host,
            [
                deploy_flow.SERVER_DEPLOY_STDOUT,
                Path(str(deploy_flow.SERVER_DEPLOY_STDOUT) + ".err"),
            ],
        )

    server_host_ip = server_host_ipv4
    client_host_ip = client_host_ipv4
    trojan_user = "deploy-suite@example.com"
    trojan_password = "deploy-pass-123"

    client_proc = None
    server_proc = None
    try:
        with deploy_flow.timed("start client deploy"):
            client_proc = deploy_flow.start_client_deploy(
                client_host,
                remote_host=server_host_ip,
                deploy_port=deploy_flow.DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=deploy_flow.TROJAN_PORT,
            )
        with deploy_flow.timed("wait client deploy link"):
            link = deploy_flow.wait_for_client_link(client_host, client_proc)
        assert link.startswith("trojan://"), "xp2p client deploy did not emit connection link"

        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address="Any",
            port=int(deploy_flow.DEPLOY_PORT),
            action="Allow",
        )
        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address="Any",
            port=int(deploy_flow.TROJAN_PORT),
            action="Allow",
        )
        with deploy_flow.timed("start server deploy"):
            server_proc = deploy_flow.start_server_deploy(
                server_host,
                listen_addr=f":{deploy_flow.DEPLOY_PORT}",
                deploy_link=link,
            )

        with deploy_flow.timed("wait server deploy logs"):
            initial_server_log = deploy_flow.wait_for_any_log_phrase(
                server_host,
                server_proc,
                [
                    "server deploy: manifest decrypted",
                    "server deploy: starting xray-core",
                    "server deploy: starting listener",
                ],
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
            if initial_server_log == "server deploy: starting listener":
                deploy_flow.wait_for_log_phrase(
                    server_host,
                    server_proc,
                    "server deploy: manifest decrypted",
                    timeout=deploy_flow.LOG_WAIT_TIMEOUT,
                )
            if initial_server_log != "server deploy: starting xray-core":
                deploy_flow.wait_for_log_phrase(
                    server_host,
                    server_proc,
                    "server deploy: starting xray-core",
                    timeout=deploy_flow.LOG_WAIT_TIMEOUT,
                )
        with deploy_flow.timed("wait client deploy logs"):
            deploy_flow.wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: connection link received",
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
            deploy_flow.wait_for_log_phrase(
                client_host,
                client_proc,
                "client deploy: local install completed",
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
            deploy_flow.wait_for_any_log_phrase(
                client_host,
                client_proc,
                [
                    "client deploy: completed",
                    "client deploy: client run active",
                ],
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
        with deploy_flow.timed("wait server deploy completion"):
            deploy_flow.wait_for_any_log_phrase(
                server_host,
                server_proc,
                [
                    "server deploy: completion requested",
                    "server deploy: stopped",
                ],
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
        if client_proc:
            deploy_flow.stop_process(client_host, client_proc["pid"])
            client_proc = None
        if server_proc:
            deploy_flow.stop_process(server_host, server_proc["pid"])
            server_proc = None
        with deploy_flow.timed("start xp2p services"):
            xp2p_server_runner("server", "service", "start", check=True)
            xp2p_client_runner("client", "service", "start", check=True)
        with deploy_flow.timed("wait apply.request clear"):
            apply_flow.wait_for_apply_request_clear(
                server_host,
                timeout=90.0,
                poll_seconds=1.0,
                dump_label="server-deploy-apply-timeout",
            )
            apply_flow.wait_for_apply_request_clear(
                client_host,
                timeout=90.0,
                poll_seconds=1.0,
                dump_label="client-deploy-apply-timeout",
            )

        with deploy_flow.timed("check client internet access"):
            deploy_assert.assert_internet_access(client_host)

        with deploy_flow.timed("assert client artifacts"):
            deploy_assert.assert_client_install_artifacts(
                client_host, server_host_ip, trojan_user, trojan_password
            )
        with deploy_flow.timed("assert client state"):
            deploy_assert.assert_client_state(client_host, server_host_ip)
        with deploy_flow.timed("assert client routing"):
            deploy_assert.assert_client_routing(client_host, server_host_ip)

        with deploy_flow.timed("wait heartbeat state"):
            heartbeat = deploy_assert.wait_for_heartbeat_state(client_host, timeout=deploy_flow.LOG_WAIT_TIMEOUT)
        with deploy_flow.timed("assert heartbeat entry"):
            deploy_assert.assert_heartbeat_entry(
                heartbeat,
                deploy_assert.expected_tag(server_host_ip),
                host=server_host_ip,
                user=trojan_user,
                client_ip=client_host_ip,
            )
    except pytest.skip.Exception:
        raise
    except Exception:
        win_env.dump_failure_state(client_host, label="client-deploy-end-to-end")
        win_env.dump_failure_state(server_host, label="server-deploy-end-to-end")
        raise
    finally:
        total = time.perf_counter() - test_start
        print(f"TIMING: test_windows_client_deploy_end_to_end total: {total:.2f}s")
        if client_proc:
            deploy_flow.stop_process(client_host, client_proc["pid"])
        if server_proc:
            deploy_flow.stop_process(server_host, server_proc["pid"])
        deploy_flow.stop_xp2p_processes(client_host)
        deploy_flow.stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address="Any",
            port=int(deploy_flow.DEPLOY_PORT),
            action="Allow",
        )
        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address="Any",
            port=int(deploy_flow.TROJAN_PORT),
            action="Allow",
        )
        for host in (client_host, server_host):
            deploy_flow.remove_paths(host, deploy_assert.HEARTBEAT_STATE_FILES)


@pytest.mark.host
@pytest.mark.win
def test_windows_server_deploy_falls_back_to_self_signed_on_invalid_cert(
    client_host,
    server_host,
    client_host_ipv4,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
    xp2p_msi_path,
):
    deploy_flow.stop_xp2p_processes(client_host)
    deploy_flow.stop_xp2p_processes(server_host)
    xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
    xp2p_server_runner("server", "remove", "--ignore-missing")
    deploy_flow.remove_paths(client_host, [deploy_assert.CLIENT_CONFIG_DIR, *deploy_assert.CLIENT_STATE_FILES])
    deploy_flow.remove_paths(server_host, [deploy_assert.SERVER_CONFIG_DIR, *deploy_assert.SERVER_STATE_FILES])

    for host in (client_host, server_host):
        deploy_flow.remove_paths(host, deploy_assert.HEARTBEAT_STATE_FILES)
    deploy_flow.remove_paths(
        client_host,
        [
            deploy_flow.CLIENT_DEPLOY_STDOUT,
            Path(str(deploy_flow.CLIENT_DEPLOY_STDOUT) + ".err"),
        ],
    )
    deploy_flow.remove_paths(
        server_host,
        [
            deploy_flow.SERVER_DEPLOY_STDOUT,
            Path(str(deploy_flow.SERVER_DEPLOY_STDOUT) + ".err"),
        ],
    )

    server_host_ip = server_host_ipv4
    trojan_user = "deploy-invalid-cert@example.com"
    trojan_password = "deploy-invalid-cert-pass"
    bad_cert = Path(r"C:\Windows\Temp\xp2p-invalid-cert.pem")
    bad_key = Path(r"C:\Windows\Temp\xp2p-invalid-key.pem")

    client_proc = None
    server_proc = None
    try:
        deploy_flow.remove_path(server_host, bad_cert)
        deploy_flow.remove_path(server_host, bad_key)

        client_proc = deploy_flow.start_client_deploy(
            client_host,
            remote_host=server_host_ip,
            deploy_port=deploy_flow.DEPLOY_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=deploy_flow.TROJAN_PORT,
        )
        link = deploy_flow.wait_for_client_link(client_host, client_proc)

        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Present",
            remote_address=client_host_ipv4,
            port=int(deploy_flow.DEPLOY_PORT),
            action="Allow",
        )
        server_proc = deploy_flow.start_server_deploy_with_args(
            server_host,
            listen_addr=f":{deploy_flow.DEPLOY_PORT}",
            deploy_link=link,
            env_overrides={
                "XP2P_SERVER_CERTIFICATE": str(bad_cert),
                "XP2P_SERVER_KEY": str(bad_key),
            },
        )

        initial_server_log = deploy_flow.wait_for_any_log_phrase(
            server_host,
            server_proc,
            [
                "server deploy: manifest decrypted",
                "server deploy: starting xray-core",
            ],
            timeout=deploy_flow.LOG_WAIT_TIMEOUT,
        )
        deploy_flow.wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: certificate validation failed, using self-signed",
            timeout=deploy_flow.LOG_WAIT_TIMEOUT,
        )
        if initial_server_log != "server deploy: starting xray-core":
            deploy_flow.wait_for_log_phrase(
                server_host,
                server_proc,
                "server deploy: starting xray-core",
                timeout=deploy_flow.LOG_WAIT_TIMEOUT,
            )
        deploy_flow.wait_for_log_phrase(
            client_host,
            client_proc,
            "client deploy: local install completed",
            timeout=deploy_flow.LOG_WAIT_TIMEOUT,
        )

        if client_proc:
            deploy_flow.stop_process(client_host, client_proc["pid"])
            client_proc = None
        if server_proc:
            deploy_flow.stop_process(server_host, server_proc["pid"])
            server_proc = None

        xp2p_server_runner("server", "service", "start", check=True)
        apply_flow.wait_for_apply_request_clear(
            server_host,
            timeout=90.0,
            poll_seconds=1.0,
            dump_label="server-deploy-invalid-cert-apply-timeout",
        )

        server_xray = remote_files.read_remote_json(server_host, deploy_assert.SERVER_LIVE_XRAY_JSON)
        trojan = deploy_assert.find_trojan_inbound(server_xray)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates after deploy fallback"
        primary = certificates[0]
        cert_value = primary.get("certificateFile")
        key_value = primary.get("keyFile")
        cert_path = deploy_assert.normalize_windows_path(cert_value)
        key_path = deploy_assert.normalize_windows_path(key_value)
        expected_cert_paths = {
            deploy_assert.normalize_windows_path(str(deploy_assert.SERVER_CERT_DEST)),
            deploy_assert.normalize_windows_path(str(win_env.pending_candidate(deploy_assert.SERVER_CERT_DEST))),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_LIVE_ROOT / "config-server" / "cert.pem")),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_PENDING_ROOT / "config-server" / "cert.pem")),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_ROOT / "tls" / "server" / "cert.pem")),
            deploy_assert.normalize_windows_path(str(bad_cert)),
        }
        expected_key_paths = {
            deploy_assert.normalize_windows_path(str(deploy_assert.SERVER_KEY_DEST)),
            deploy_assert.normalize_windows_path(str(win_env.pending_candidate(deploy_assert.SERVER_KEY_DEST))),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_LIVE_ROOT / "config-server" / "key.pem")),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_PENDING_ROOT / "config-server" / "key.pem")),
            deploy_assert.normalize_windows_path(str(win_env.CONFIG_ROOT / "tls" / "server" / "key.pem")),
            deploy_assert.normalize_windows_path(str(bad_key)),
        }
        assert cert_path in expected_cert_paths
        assert key_path in expected_key_paths
        assert win_env.path_exists(server_host, cert_value), f"certificateFile not found: {cert_value}"
        assert win_env.path_exists(server_host, key_value), f"keyFile not found: {key_value}"
    except Exception:
        win_env.dump_failure_state(client_host, label="client-deploy-invalid-cert")
        win_env.dump_failure_state(server_host, label="server-deploy-invalid-cert")
        raise
    finally:
        if client_proc:
            deploy_flow.stop_process(client_host, client_proc["pid"])
        if server_proc:
            deploy_flow.stop_process(server_host, server_proc["pid"])
        deploy_flow.stop_xp2p_processes(client_host)
        deploy_flow.stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        xp2p_server_runner("server", "remove", "--ignore-missing")
        deploy_flow.set_firewall_rule(
            server_host,
            ensure="Absent",
            remote_address=client_host_ipv4,
            port=int(deploy_flow.DEPLOY_PORT),
            action="Allow",
        )
        for host in (client_host, server_host):
            deploy_flow.remove_paths(host, deploy_assert.HEARTBEAT_STATE_FILES)
