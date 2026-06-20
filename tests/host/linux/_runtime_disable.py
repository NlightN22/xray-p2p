from __future__ import annotations

import time
from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 1.0
APPLY_REQUEST = helpers.STATE_ROOT / "apply.request"
APPLY_ERROR = helpers.STATE_ROOT / "apply.error"


def xp2p_runner(host):
    def _run(*args: str, check: bool = False):
        result = linux_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            helpers.dump_failure_state(host, f"runtime-disable-{host.backend.hostname}")
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def start_service(host, runner, role: str) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    runner(role, "service", "stop")
    host.run("sudo -n pkill -f '/etc/xp2p/bin/[x]ray' >/dev/null 2>&1 || true")
    runner(role, "service", "start", check=True)
    wait_for_service(host, role, active=True)
    wait_for_live_xray(host, role)
    assert_apply_clean(host)


def stop_service(runner, role: str) -> None:
    runner(role, "service", "stop")


def restart_service(host, runner, role: str) -> None:
    runner(role, "service", "restart", check=True)
    wait_for_service(host, role, active=True)
    wait_for_live_xray(host, role)


def wait_for_service(host, role: str, *, active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last = None
    while time.time() < deadline:
        last = host.run(f"sudo -n systemctl is-active xp2p-{role}.service")
        if (last.rc == 0) is active:
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if active else "inactive"
    raise AssertionError(f"xp2p-{role}.service did not become {state}: {last.stdout}\n{last.stderr}")


def wait_for_apply_clear(host) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        if not linux_env.path_exists(host, APPLY_REQUEST):
            return
        time.sleep(POLL_INTERVAL)
    helpers.dump_failure_state(host, f"apply-not-cleared-{host.backend.hostname}")
    raise AssertionError("apply.request did not clear")


def wait_for_live_xray(host, role: str) -> dict:
    path = live_dir(role) / "xray.json"
    deadline = time.time() + SERVICE_TIMEOUT
    last_error: Exception | None = None
    while time.time() < deadline:
        if linux_env.path_exists(host, path):
            try:
                return helpers.read_json(host, path)
            except RuntimeError as exc:
                last_error = exc
        time.sleep(POLL_INTERVAL)
    if last_error:
        raise AssertionError(f"Failed to read live xray config {path}: {last_error}") from last_error
    raise AssertionError(f"Live xray config {path} was not published")


def live_dir(role: str) -> PurePosixPath:
    if role == "client":
        return helpers.CLIENT_LIVE_DIR
    return helpers.SERVER_LIVE_DIR


def xray_pid(host) -> str:
    result = host.run("pgrep -f '/etc/xp2p/bin/xray' | head -n1")
    pid = (result.stdout or "").strip()
    if result.rc != 0 or not pid:
        helpers.dump_failure_state(host, f"xray-pid-missing-{host.backend.hostname}")
        raise AssertionError(f"xray process not found.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")
    return pid


def wait_for_stable_xray_pid(host) -> str:
    deadline = time.time() + SERVICE_TIMEOUT
    previous = ""
    while time.time() < deadline:
        current = xray_pid(host)
        if current == previous:
            return current
        previous = current
        time.sleep(POLL_INTERVAL)
    helpers.dump_failure_state(host, f"xray-pid-unstable-{host.backend.hostname}")
    raise AssertionError(f"xray PID did not become stable, last PID: {previous}")


def assert_apply_clean(host) -> None:
    if linux_env.path_exists(host, APPLY_REQUEST):
        helpers.dump_failure_state(host, f"apply-request-left-{host.backend.hostname}")
        raise AssertionError("apply.request should be removed")
    if linux_env.path_exists(host, APPLY_ERROR):
        helpers.dump_failure_state(host, f"apply-error-left-{host.backend.hostname}")
        raise AssertionError("apply.error should not exist")


def assert_same_xray_pid(host, expected: str, label: str) -> None:
    actual = xray_pid(host)
    if actual != expected:
        helpers.dump_failure_state(host, label)
        raise AssertionError(f"xray PID changed: expected {expected}, got {actual}")


def trojan_user_ids(xray: dict) -> set[str]:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("protocol") != "trojan":
            continue
        clients = inbound.get("settings", {}).get("clients") or []
        return {str(client.get("email")) for client in clients if client.get("email")}
    raise AssertionError("Trojan inbound not found in live xray config")


def outbound_tags(xray: dict) -> set[str]:
    return {str(outbound.get("tag")) for outbound in xray.get("outbounds") or [] if outbound.get("tag")}


def route_outbound_targets(xray: dict, outbound_tag: str) -> list[dict]:
    rules = xray.get("routing", {}).get("rules") or []
    return [rule for rule in rules if isinstance(rule, dict) and rule.get("outboundTag") == outbound_tag]


def assert_no_route_to(xray: dict, outbound_tag: str) -> None:
    rules = route_outbound_targets(xray, outbound_tag)
    if rules:
        raise AssertionError(f"Unexpected routing rules for {outbound_tag}: {rules}")


def assert_client_reverse_bridge_without_rules(xray: dict, reverse_tag: str) -> None:
    bridges = xray.get("reverse", {}).get("bridges") or []
    if not any(entry.get("tag") == reverse_tag for entry in bridges if isinstance(entry, dict)):
        raise AssertionError(f"Reverse bridge {reverse_tag} missing from live xray config")
    rules = xray.get("routing", {}).get("rules") or []
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        inbound = rule.get("inboundTag") or []
        domains = rule.get("domain") or []
        if reverse_tag in inbound or f"full:{reverse_tag}" in domains:
            raise AssertionError(f"Unexpected reverse inbound rule for {reverse_tag}: {rule}")
