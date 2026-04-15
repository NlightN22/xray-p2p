from __future__ import annotations

import shlex
import time
from pathlib import PurePosixPath

import pytest
from testinfra.host import Host

from tests.host.cross import helpers_linux as cross_linux
from tests.host.linux import _helpers as helpers
from tests.host.linux import dual_deploy_helpers as deploy_helpers
from tests.host.linux import env as linux_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

TROJAN_PORT = "58641"
LOG_WAIT_TIMEOUT = 60
APPLY_WAIT_TIMEOUT = 90.0
SERVICE_TIMEOUT = 45.0
POLL_INTERVAL = 1.5


def test_backup_restore_after_full_wipe(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    run_id = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    client_log = PurePosixPath(f"/tmp/xp2p-backup-restore-client-{run_id}.log")
    server_log = PurePosixPath(f"/tmp/xp2p-backup-restore-server-{run_id}.log")
    client_archive = PurePosixPath(f"/tmp/xp2p-client-backup-{run_id}.tar.gz")
    server_archive = PurePosixPath(f"/tmp/xp2p-server-backup-{run_id}.tar.gz")

    client_ip = cross_linux.detect_linux_ipv4_non_nat(client_host)
    server_ip = cross_linux.detect_linux_ipv4_non_nat(server_host)

    try:
        deploy_helpers.deploy_client_to_server(
            client_host,
            server_host,
            client_log=client_log,
            server_log=server_log,
            server_ip=server_ip,
            trojan_user="backup-restore@example.com",
            trojan_password="backup-restore-secret",
            trojan_port=TROJAN_PORT,
            log_wait_timeout=LOG_WAIT_TIMEOUT,
        )

        xp2p_server_runner("server", "service", "start", check=True)
        xp2p_client_runner("client", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        _wait_for_diag_listener(client_host, helpers.CLIENT_DIAG_PORT)
        _wait_for_diag_listener(server_host, helpers.SERVER_DIAG_PORT)

        helpers.assert_tunnel_ping_bidirectional(
            xp2p_client_runner,
            xp2p_server_runner,
            client_ip=client_ip,
            server_ip=server_ip,
            label="before backup",
        )

        xp2p_client_runner(
            "client",
            "export",
            "--config-root",
            helpers.CONFIG_ROOT.as_posix(),
            "--output",
            client_archive.as_posix(),
            check=True,
        )
        xp2p_server_runner(
            "server",
            "export",
            "--config-root",
            helpers.CONFIG_ROOT.as_posix(),
            "--output",
            server_archive.as_posix(),
            check=True,
        )
        assert linux_env.path_exists(client_host, client_archive), f"client archive missing: {client_archive}"
        assert linux_env.path_exists(server_host, server_archive), f"server archive missing: {server_archive}"

        _stop_role_service_and_wait_inactive(client_host, "client", xp2p_client_runner)
        _stop_role_service_and_wait_inactive(server_host, "server", xp2p_server_runner)
        _wipe_installation_roots(client_host)
        _wipe_installation_roots(server_host)
        assert not linux_env.path_exists(client_host, helpers.CONFIG_ROOT), f"{helpers.CONFIG_ROOT} should be removed"
        assert not linux_env.path_exists(server_host, helpers.CONFIG_ROOT), f"{helpers.CONFIG_ROOT} should be removed"

        xp2p_client_runner(
            "client",
            "import",
            "--config-root",
            helpers.CONFIG_ROOT.as_posix(),
            "--input",
            client_archive.as_posix(),
            check=True,
        )
        xp2p_server_runner(
            "server",
            "import",
            "--config-root",
            helpers.CONFIG_ROOT.as_posix(),
            "--input",
            server_archive.as_posix(),
            check=True,
        )

        _assert_import_restored_root(client_host, role="client")
        _assert_import_restored_root(server_host, role="server")

        xp2p_server_runner("server", "service", "start", check=True)
        xp2p_client_runner("client", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        _wait_for_path(client_host, helpers.CONFIG_LIVE_ROOT / "xp2p-client.toml")
        _wait_for_path(server_host, helpers.CONFIG_LIVE_ROOT / "xp2p-server.toml")
        _wait_for_diag_listener(client_host, helpers.CLIENT_DIAG_PORT)
        _wait_for_diag_listener(server_host, helpers.SERVER_DIAG_PORT)

        helpers.assert_tunnel_ping_bidirectional(
            xp2p_client_runner,
            xp2p_server_runner,
            client_ip=client_ip,
            server_ip=server_ip,
            label="after restore",
        )
    except BaseException:
        _dump_archive_debug(client_host, client_archive, role="client")
        _dump_archive_debug(server_host, server_archive, role="server")
        helpers.dump_failure_state(client_host, "backup-restore-client")
        helpers.dump_failure_state(server_host, "backup-restore-server")
        raise
    finally:
        deploy_helpers.stop_services(client_host, server_host)


def _wait_for_diag_listener(host: Host, port: str) -> None:
    port_value = str(int(port))
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        result = host.run(f"sudo -n ss -lntp | grep :{port_value}")
        if result.rc == 0:
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Expected diagnostics listener on port {port} after {SERVICE_TIMEOUT} seconds.")


def _stop_role_service_and_wait_inactive(host: Host, role: str, runner) -> None:
    runner(role, "service", "stop")
    unit = f"xp2p-{role}.service"
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        if host.run(f"sudo -n systemctl is-active {shlex.quote(unit)} >/dev/null 2>&1").rc != 0:
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Expected {unit} to stop within {SERVICE_TIMEOUT} seconds.")


def _wipe_installation_roots(host: Host) -> None:
    root_arg = shlex.quote(helpers.CONFIG_ROOT.as_posix())
    log_arg = shlex.quote(helpers.LOG_ROOT.as_posix())
    script = (
        "rm -rf -- \"$1\" \"$2\"; "
        "for path in \"$1\".bak-*; do [ -e \"$path\" ] || continue; rm -rf \"$path\"; done"
    )
    host.run(f"sudo -n /bin/sh -c {shlex.quote(script)} -- {root_arg} {log_arg}")


def _assert_import_restored_root(host: Host, *, role: str) -> None:
    if not linux_env.path_exists(host, helpers.CONFIG_ROOT):
        raise AssertionError(f"{role} import did not restore config root at {helpers.CONFIG_ROOT}")


def _wait_for_path(host: Host, path: PurePosixPath) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        if linux_env.path_exists(host, path):
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Expected {path} to exist after {SERVICE_TIMEOUT} seconds.")


def _dump_archive_debug(host: Host, path: PurePosixPath, *, role: str) -> None:
    quoted_path = shlex.quote(path.as_posix())
    script = (
        f"echo '--- {role} archive ---'; "
        f"ls -la {quoted_path} 2>/dev/null || true; "
        f"echo '--- {role} archive list (first 200) ---'; "
        f"tar -tzf {quoted_path} 2>/dev/null | sed -n '1,200p' || true; "
        "true"
    )
    result = host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
    if result.stdout:
        print(result.stdout, flush=True)
    if result.stderr:
        print(result.stderr, flush=True)
