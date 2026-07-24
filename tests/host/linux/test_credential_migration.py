from __future__ import annotations

import json
import re
import time

import pytest

from tests.host import _credential_migration as credentials
from tests.host.linux import _helpers as helpers
from tests.host.linux import _credential_migration_upgrade as upgrade
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux import env as linux_env


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.serial]

CONTROL_PORT = "62022"
TROJAN_PORT = "62310"
SERVER_HOST = "credential-migration.example.test"
USER = "migration-client@example.test"
OLD_CREDENTIAL = "550e8400-e29b-41d4-a716-446655440010"
HOSTS_MARKER = "# xp2p-credential-migration"
RUNTIME_META = helpers.CLIENT_LIVE_DIR / "runtime.json"
CLIENT_LOG = helpers.LOG_ROOT / "client" / "service.log"


def test_client_migrates_old_live_credential_and_survives_restart(
    client_host, server_host
):
    server_runner, client_runner = _install_previous_release_client_state(
        client_host, server_host
    )
    server_ip = upgrade.detect_host_ipv4(server_host)
    try:
        runtime.start_service(server_host, server_runner, "server")
        runtime.start_service(client_host, client_runner, "client", log_level="debug")
        _assert_tunnel_ping(client_host, server_host, client_runner)
        _assert_client_converged(client_host, OLD_CREDENTIAL)

        runtime.stop_service(client_runner, "client")
        _clear_client_log(client_host)

        server_runner("server", "user", "rotate", USER, check=True)
        rotated = _server_user(server_host)
        active = rotated["active_credential"]
        assert active != OLD_CREDENTIAL
        assert rotated["previous_credential_for_rotation"] == OLD_CREDENTIAL
        _assert_client_converged(client_host, OLD_CREDENTIAL)
        subscription_generation = _server_subscription_generation(server_host)

        upgrade.install_deb(client_host, upgrade.current_candidate_deb())
        runtime.start_service(client_host, client_runner, "client", log_level="debug")
        _wait_for_client_convergence(client_host, active)
        _wait_for_acknowledgement(server_host)
        _assert_runtime_migration_completed(
            client_host, server_host, client_runner, subscription_generation
        )
        converged_state = _client_credential_state(client_host)
        acknowledged_state = _server_user(server_host)

        runtime.stop_service(client_runner, "client")
        runtime.stop_service(server_runner, "server")
        runtime.start_service(server_host, server_runner, "server")
        runtime.start_service(client_host, client_runner, "client", log_level="debug")

        _wait_for_client_convergence(client_host, active)
        _assert_running_client_uses_active_credential(
            client_host, server_host, client_runner
        )
        assert _client_credential_state(client_host) == converged_state
        assert _server_user(server_host) == acknowledged_state
        assert upgrade.detect_host_ipv4(server_host) == server_ip
    finally:
        runtime.stop_service(client_runner, "client")
        runtime.stop_service(server_runner, "server")
        upgrade.remove_hosts_entry(client_host, HOSTS_MARKER)


def test_runtime_apply_failure_keeps_old_state_and_defers_acknowledgement(
    client_host, server_host
):
    server_runner, client_runner = _install_old_client_state(client_host, server_host)
    try:
        runtime.start_service(server_host, server_runner, "server")
        runtime.start_service(client_host, client_runner, "client", log_level="debug")
        _assert_tunnel_ping(client_host, server_host, client_runner)
        _assert_client_converged(client_host, OLD_CREDENTIAL)

        _rewrite_live_api_listen(client_host, "127.0.0.1:1")
        server_runner("server", "user", "rotate", USER, check=True)
        active = _server_user(server_host)["active_credential"]

        _wait_for_subscription_apply_failure(client_host)
        _assert_client_converged(client_host, OLD_CREDENTIAL)
        server_state = _server_user(server_host)
        assert server_state["active_credential"] == active
        assert server_state["previous_credential_for_rotation"] == OLD_CREDENTIAL
        _assert_tunnel_ping(client_host, server_host, client_runner)
    finally:
        runtime.stop_service(client_runner, "client")
        runtime.stop_service(server_runner, "server")
        upgrade.remove_hosts_entry(client_host, HOSTS_MARKER)


def test_server_start_preserves_user_credentials(server_host):
    runner = runtime.xp2p_runner(server_host)
    try:
        runner(
            "server", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--host", SERVER_HOST,
            "--port", TROJAN_PORT, "--force", check=True,
        )
        runner(
            "server", "user", "add", "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--id", USER,
            "--password", "synthetic-legacy-credential", "--host", SERVER_HOST,
            check=True,
        )
        before = _server_user(server_host)

        runtime.start_service(server_host, runner, "server")
        after = _server_user(server_host)

        assert after == before
        live = runtime.wait_for_live_xray(server_host, "server")
        assert credentials.xray_inbound_credential(live, USER) == "synthetic-legacy-credential"
    finally:
        runtime.stop_service(runner, "server")


def _install_old_client_state(client_host, server_host):
    server_runner = runtime.xp2p_runner(server_host)
    client_runner = runtime.xp2p_runner(client_host)
    server_ip = upgrade.detect_host_ipv4(server_host)
    upgrade.add_hosts_entry(client_host, server_ip, SERVER_HOST, HOSTS_MARKER)
    server_runner(
        "server", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
        "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--host", SERVER_HOST,
        "--port", TROJAN_PORT, "--force", check=True,
    )
    added = server_runner(
        "server", "user", "add", "--json", "--path", helpers.INSTALL_ROOT.as_posix(),
        "--config-dir", helpers.SERVER_CONFIG_DIR_NAME, "--id", USER,
        "--password", OLD_CREDENTIAL, "--host", SERVER_HOST, check=True,
    )
    link = credentials.connection_link(added.stdout or "")
    client_runner(
        "client", "install", "--path", helpers.INSTALL_ROOT.as_posix(),
        "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME, "--link", link,
        "--mode", "proxy", check=True,
    )
    return server_runner, client_runner


def _install_previous_release_client_state(client_host, server_host):
    previous_deb = upgrade.ensure_previous_release_deb("0.2.7")
    upgrade.install_deb(client_host, previous_deb)
    server_runner, client_runner = _install_old_client_state(client_host, server_host)
    return server_runner, client_runner


def _assert_client_converged(host, expected: str) -> None:
    credentials.assert_client_persisted_credential_converged(
        helpers.read_pending_client_config(host),
        helpers.read_json(host, RUNTIME_META),
        runtime.wait_for_live_xray(host, "client"),
        USER,
        expected,
    )


def _client_credential_state(host) -> dict:
    desired = helpers.read_pending_client_config(host)
    runtime_meta = helpers.read_json(host, RUNTIME_META)
    live_xray = runtime.wait_for_live_xray(host, "client")
    return {
        "desired": credentials.client_endpoint(desired, USER),
        "runtime": credentials.runtime_endpoint(runtime_meta, USER),
        "live_xray_artifact": credentials.xray_outbound_credential(live_xray, USER),
    }


def _wait_for_client_convergence(host, expected: str) -> None:
    def read_state():
        try:
            _assert_client_converged(host, expected)
            return True
        except (AssertionError, RuntimeError, json.JSONDecodeError):
            return False

    credentials.wait_until(
        read_state, bool, timeout=100.0,
        description=f"client credential convergence to {expected}",
    )


def _server_user(host) -> dict:
    return credentials.server_user(helpers.read_pending_server_config(host), USER)


def _wait_for_acknowledgement(host) -> None:
    credentials.wait_until(
        lambda: _server_user(host),
        lambda state: not state.get("previous_credential_for_rotation"),
        timeout=60.0,
        description="server credential acknowledgement",
    )


def _assert_tunnel_ping(client_host, server_host, runner) -> None:
    def probe():
        result = runner("ping", SERVER_HOST, "-T", "--count", "2")
        return result.rc, result.stdout or "", result.stderr or ""

    try:
        credentials.wait_until(
            probe,
            lambda result: result[0] == 0 and "0% loss" in result[1].lower(),
            timeout=45.0,
            description="working tunnel after credential migration",
        )
    except AssertionError:
        helpers.dump_failure_state(client_host, "credential-migration-ping")
        helpers.dump_failure_state(server_host, "credential-migration-ping")
        raise


def _assert_runtime_migration_completed(
    client_host, server_host, runner, subscription_generation: str
) -> None:
    _assert_running_client_uses_active_credential(client_host, server_host, runner)
    log = helpers.read_text(client_host, CLIENT_LOG)
    apply_matches = re.findall(
        r"runtime outbound apply completed.*request_id[=: ]+([^ ]+)", log, re.I
    )
    runtime_applied = log.lower().find("runtime outbound apply completed")
    publication_match = re.search(
        rf"subscription applied.*generation[=: ]+{re.escape(subscription_generation)}",
        log,
        re.I,
    )
    published = publication_match.start() if publication_match else -1
    assert len(apply_matches) == 1 and apply_matches[0].strip(), (
        f"Expected one runtime apply for this migration, got {apply_matches}"
    )
    assert published > runtime_applied >= 0, (
        "Current subscription generation was published/acknowledged before runtime apply"
    )


def _assert_running_client_uses_active_credential(
    client_host, server_host, runner
) -> None:
    _assert_previous_rejected(server_host, OLD_CREDENTIAL)
    _assert_tunnel_ping(client_host, server_host, runner)


def _clear_client_log(host) -> None:
    host.run(f"sudo -n sh -c ': > {CLIENT_LOG}'")


def _server_subscription_generation(host) -> str:
    runtime_meta = helpers.read_json(host, helpers.SERVER_LIVE_DIR / "runtime.json")
    generation = str(
        runtime_meta.get("control", {}).get("subscription", {}).get("generation") or ""
    )
    assert generation, "Server subscription generation is missing"
    return generation


def _assert_previous_rejected(host, credential: str) -> None:
    result = linux_env.run_guest_script(
        host, "scripts/linux/check_credential_rotation_rejected.sh",
        "127.0.0.1", CONTROL_PORT, USER, credential, timeout=60,
    )
    assert result.rc == 0, f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"


def _wait_for_subscription_apply_failure(host) -> None:
    log = helpers.LOG_ROOT / "client" / "service.log"
    credentials.wait_until(
        lambda: helpers.read_text(host, log) if helpers.path_exists(host, log) else "",
        lambda text: "subscription apply failed" in str(text).lower(),
        timeout=45.0,
        description="failed credential runtime apply",
    )


def _rewrite_live_api_listen(host, listen: str) -> None:
    path = helpers.CLIENT_LIVE_DIR / "xray.json"
    data = helpers.read_json(host, path)
    data.setdefault("api", {})["listen"] = listen
    helpers.write_text(host, path, json.dumps(data, indent=2) + "\n")
