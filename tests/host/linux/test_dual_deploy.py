from __future__ import annotations

import os
import time

import pytest
from _pytest.outcomes import OutcomeException
from tests.host.cross import helpers_linux as cross_linux
from tests.host.linux import _helpers as helpers
from tests.host.linux import dual_deploy_helpers as deploy_helpers
from tests.host.linux import env as linux_env

pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
]

TROJAN_PORT = "58621"
LOG_WAIT_TIMEOUT = 60
APPLY_WAIT_TIMEOUT = 90
LOG_ROOT = linux_env.WORK_TREE / "build" / "logs" / "linux"
CLIENT_SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
SERVER_SERVICE_LOG = helpers.LOG_ROOT / "server" / "service.log"


@pytest.mark.host
@pytest.mark.linux
def test_dual_client_deploy_with_server_service_running(request, client_host, server_host, aux_host):
    _skip_if_not_selected(request)
    server_runner = cross_linux.linux_runner(server_host)
    client_a_runner = cross_linux.linux_runner(client_host)
    client_b_runner = cross_linux.linux_runner(aux_host)

    run_id = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    server_ip = cross_linux.detect_linux_ipv4_non_nat(server_host)
    user_a = "dual-client-a@example.com"
    pass_a = "dual-client-a-pass"
    user_b = "dual-client-b@example.com"
    pass_b = "dual-client-b-pass"

    for host in (client_host, server_host, aux_host):
        deploy_helpers.ensure_log_dir(host, LOG_ROOT)

    try:
        deploy_helpers.deploy_client_to_server(
            client_host,
            server_host,
            client_log=LOG_ROOT / f"dual-client-a-{run_id}.log",
            server_log=LOG_ROOT / f"dual-server-a-{run_id}.log",
            server_ip=server_ip,
            trojan_user=user_a,
            trojan_password=pass_a,
            trojan_port=TROJAN_PORT,
            log_wait_timeout=LOG_WAIT_TIMEOUT,
        )
        client_a_runner("client", "service", "start", check=True)
        server_runner("server", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(server_runner, "server")
        server_log_offset = deploy_helpers.log_offset(server_host, SERVER_SERVICE_LOG)
        deploy_helpers.assert_deploy_service_hint(
            server_host,
            LOG_ROOT / f"dual-server-a-{run_id}.log",
            "start",
        )

        deploy_helpers.deploy_client_to_server(
            aux_host,
            server_host,
            client_log=LOG_ROOT / f"dual-client-b-{run_id}.log",
            server_log=LOG_ROOT / f"dual-server-b-{run_id}.log",
            server_ip=server_ip,
            trojan_user=user_b,
            trojan_password=pass_b,
            trojan_port=str(int(TROJAN_PORT) + 1),
            log_wait_timeout=LOG_WAIT_TIMEOUT,
        )
        client_b_runner("client", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(aux_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(server_runner, "server")
        deploy_helpers.assert_no_service_stop(server_host, SERVER_SERVICE_LOG, server_log_offset, "server")
        deploy_helpers.assert_deploy_service_hint(
            server_host,
            LOG_ROOT / f"dual-server-b-{run_id}.log",
            "restart",
        )
        deploy_helpers.assert_log_phrase_present(
            server_host,
            LOG_ROOT / f"dual-server-b-{run_id}.log",
            "server deploy: service active; skipping xray-core start",
        )
        deploy_helpers.assert_log_phrase_absent(
            server_host,
            LOG_ROOT / f"dual-server-b-{run_id}.log",
            "server deploy: starting xray-core",
        )
        deploy_helpers.restart_service(server_host, "server")
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(server_runner, "server")

        deploy_helpers.assert_server_state_reports_users(server_host, {user_a, user_b})
        deploy_helpers.assert_client_endpoints(client_host, {server_ip})
        deploy_helpers.assert_client_endpoints(aux_host, {server_ip})
        deploy_helpers.assert_ping_ok(client_a_runner, server_ip)
        deploy_helpers.assert_ping_ok(client_b_runner, server_ip)
    except BaseException as exc:
        if not isinstance(exc, OutcomeException):
            raise
        helpers.dump_failure_state(server_host, "dual-client-deploy-server")
        helpers.dump_failure_state(client_host, "dual-client-deploy-client-a")
        helpers.dump_failure_state(aux_host, "dual-client-deploy-client-b")
        raise
    finally:
        deploy_helpers.stop_services(client_host, aux_host, server_host)


@pytest.mark.host
@pytest.mark.linux
def test_dual_server_deploy_with_client_service_running(request, client_host, server_host, aux_host):
    _skip_if_not_selected(request)
    client_runner = cross_linux.linux_runner(client_host)
    server_a_runner = cross_linux.linux_runner(server_host)
    server_b_runner = cross_linux.linux_runner(aux_host)

    run_id = time.strftime("%Y%m%d-%H%M%S", time.gmtime())
    server_a_ip = cross_linux.detect_linux_ipv4_non_nat(server_host)
    server_b_ip = cross_linux.detect_linux_ipv4_non_nat(aux_host)
    user_a = "dual-server-a@example.com"
    pass_a = "dual-server-a-pass"
    user_b = "dual-server-b@example.com"
    pass_b = "dual-server-b-pass"

    for host in (client_host, server_host, aux_host):
        deploy_helpers.ensure_log_dir(host, LOG_ROOT)

    try:
        deploy_helpers.deploy_client_to_server(
            client_host,
            server_host,
            client_log=LOG_ROOT / f"dual-client-a-{run_id}.log",
            server_log=LOG_ROOT / f"dual-server-a-{run_id}.log",
            server_ip=server_a_ip,
            trojan_user=user_a,
            trojan_password=pass_a,
            trojan_port=TROJAN_PORT,
            log_wait_timeout=LOG_WAIT_TIMEOUT,
        )
        client_runner("client", "service", "start", check=True)
        server_a_runner("server", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(server_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(client_runner, "client")
        client_log_offset = deploy_helpers.log_offset(client_host, CLIENT_SERVICE_LOG)
        deploy_helpers.assert_deploy_service_hint(
            client_host,
            LOG_ROOT / f"dual-client-a-{run_id}.log",
            "start",
        )
        deploy_helpers.assert_deploy_service_hint(
            server_host,
            LOG_ROOT / f"dual-server-a-{run_id}.log",
            "start",
        )

        deploy_helpers.deploy_client_to_server(
            client_host,
            aux_host,
            client_log=LOG_ROOT / f"dual-client-b-{run_id}.log",
            server_log=LOG_ROOT / f"dual-server-b-{run_id}.log",
            server_ip=server_b_ip,
            trojan_user=user_b,
            trojan_password=pass_b,
            trojan_port=str(int(TROJAN_PORT) + 1),
            log_wait_timeout=LOG_WAIT_TIMEOUT,
        )
        server_b_runner("server", "service", "start", check=True)
        deploy_helpers.wait_for_apply_request_clear(aux_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(client_runner, "client")
        deploy_helpers.assert_no_service_stop(client_host, CLIENT_SERVICE_LOG, client_log_offset, "client")
        deploy_helpers.assert_deploy_service_hint(
            client_host,
            LOG_ROOT / f"dual-client-b-{run_id}.log",
            "restart",
        )
        deploy_helpers.assert_deploy_service_hint(
            aux_host,
            LOG_ROOT / f"dual-server-b-{run_id}.log",
            "start",
        )
        deploy_helpers.restart_service(client_host, "client")
        deploy_helpers.wait_for_apply_request_clear(client_host, timeout_seconds=APPLY_WAIT_TIMEOUT)
        deploy_helpers.assert_service_active(client_runner, "client")

        deploy_helpers.assert_client_endpoints(client_host, {server_a_ip, server_b_ip})
        deploy_helpers.assert_ping_ok(client_runner, server_a_ip)
        deploy_helpers.assert_ping_ok(client_runner, server_b_ip)
    except BaseException as exc:
        if not isinstance(exc, OutcomeException):
            raise
        helpers.dump_failure_state(client_host, "dual-server-deploy-client")
        helpers.dump_failure_state(server_host, "dual-server-deploy-server-a")
        helpers.dump_failure_state(aux_host, "dual-server-deploy-server-b")
        raise
    finally:
        deploy_helpers.stop_services(client_host, aux_host, server_host)


def _skip_if_not_selected(request) -> None:
    if _should_run(request):
        return
    pytest.skip("dual deploy tests are opt-in unless explicitly selected")


def _should_run(request) -> bool:
    flag = os.environ.get("XP2P_RUN_DUAL_DEPLOY_TESTS", "").strip().lower()
    if flag in {"1", "true", "yes"}:
        return True
    args = [str(arg).replace("\\", "/") for arg in request.config.args or []]
    explicit_targets = {
        "tests/host/linux/test_dual_deploy.py",
        "test_dual_deploy.py",
        "::test_dual_client_deploy_with_server_service_running",
        "::test_dual_server_deploy_with_client_service_running",
    }
    for arg in args:
        if any(target in arg for target in explicit_targets):
            return True
    keyword = (request.config.option.keyword or "").lower()
    if "dual_deploy" in keyword:
        return True
    if "dual_client_deploy_with_server_service_running" in keyword:
        return True
    if "dual_server_deploy_with_client_service_running" in keyword:
        return True
    return False
