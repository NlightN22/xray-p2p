from __future__ import annotations

from pathlib import Path

from tests.host.win import env as _env
from tests.host.win import test_client_deploy as deploy

HEARTBEAT_STATE_FILES = deploy.HEARTBEAT_STATE_FILES


def remove_deploy_logs(host) -> None:
    _env.remove_paths(
        host,
        [
            deploy.CLIENT_DEPLOY_STDOUT,
            Path(str(deploy.CLIENT_DEPLOY_STDOUT) + ".err"),
            deploy.SERVER_DEPLOY_STDOUT,
            Path(str(deploy.SERVER_DEPLOY_STDOUT) + ".err"),
        ],
    )


def stop_deploy_process(host, proc_info: dict) -> None:
    pid = proc_info.get("pid") if isinstance(proc_info, dict) else None
    if not pid:
        return
    deploy._stop_process(host, int(pid))


def ensure_deploy_firewall_rules(host, *, trojan_port: str, ensure: str) -> None:
    deploy._set_firewall_rule(
        host,
        ensure=ensure,
        remote_address="Any",
        port=int(deploy.DEPLOY_PORT),
        action="Allow",
    )
    deploy._set_firewall_rule(
        host,
        ensure=ensure,
        remote_address="Any",
        port=int(trojan_port),
        action="Allow",
    )


def start_deploy_tunnel(
    client_host,
    server_host,
    *,
    server_host_ip: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
) -> tuple[dict, dict]:
    client_proc = deploy._start_client_deploy(
        client_host,
        remote_host=server_host_ip,
        deploy_port=deploy.DEPLOY_PORT,
        trojan_user=trojan_user,
        trojan_password=trojan_password,
        trojan_port=trojan_port,
    )
    link = deploy._wait_for_client_link(client_host, client_proc)
    if not link.startswith("trojan://"):
        raise AssertionError("xp2p client deploy did not emit trojan link")

    ensure_deploy_firewall_rules(server_host, trojan_port=trojan_port, ensure="Present")
    server_proc = deploy._start_server_deploy(
        server_host,
        listen_addr=f":{deploy.DEPLOY_PORT}",
        deploy_link=link,
    )

    initial_server_log = deploy._wait_for_any_log_phrase(
        server_host,
        server_proc,
        [
            "server deploy: manifest decrypted",
            "server deploy: starting xray-core",
            "server deploy: starting listener",
        ],
        timeout=deploy.LOG_WAIT_TIMEOUT,
    )
    if initial_server_log == "server deploy: starting listener":
        deploy._wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: manifest decrypted",
            timeout=deploy.LOG_WAIT_TIMEOUT,
            )
    if initial_server_log != "server deploy: starting xray-core":
        deploy._wait_for_log_phrase(
            server_host,
            server_proc,
            "server deploy: starting xray-core",
            timeout=deploy.LOG_WAIT_TIMEOUT,
            )
    server_status = deploy._wait_for_any_log_phrase(
        server_host,
        server_proc,
        [
            "server deploy: apply request written",
            "pending config applied",
            "server deploy: completion requested",
            "xray-core process started",
            "server deploy: xray-core start failed",
        ],
        timeout=deploy.LOG_WAIT_TIMEOUT,
    )
    if server_status == "server deploy: xray-core start failed":
        combined = deploy._read_combined_logs(server_host, server_proc)
        raise AssertionError(
            "Server deploy xray-core failed to start.\n"
            f"Logs:\n{combined}"
        )

    deploy._wait_for_log_phrase(
        client_host,
        client_proc,
        "client deploy: local install completed",
        timeout=deploy.LOG_WAIT_TIMEOUT,
    )
    return client_proc, server_proc
