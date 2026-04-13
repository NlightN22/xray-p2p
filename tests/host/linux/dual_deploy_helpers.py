from __future__ import annotations

import re
import shlex
import time
from pathlib import PurePosixPath

import pytest
from _pytest.outcomes import OutcomeException
from testinfra.host import Host

from tests.host.cross import helpers_linux as cross_linux
from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")


def install_client(
    runner,
    *,
    host: str,
    port: str,
    user: str,
    password: str,
) -> None:
    runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        host,
        "--port",
        port,
        "--user",
        user,
        "--password",
        password,
        "--force",
        check=True,
    )


def install_server(runner, host: str, *, port: str) -> None:
    runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--host",
        host,
        "--port",
        port,
        "--force",
        check=True,
    )


def set_mode(runner, role: str, config_dir: str, mode: str) -> None:
    runner(
        role,
        "mode",
        mode,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        config_dir,
        check=True,
    )


def remove_client_endpoint(runner, target: str) -> None:
    runner(
        "client",
        "remove",
        target,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--quiet",
        check=True,
    )


def ensure_log_dir(host: Host, log_root: PurePosixPath) -> None:
    script = f"mkdir -p {shlex.quote(log_root.as_posix())}"
    host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")


def stop_services(*hosts: Host) -> None:
    for host in hosts:
        linux_env.run_xp2p(host, "client", "service", "stop")
        linux_env.run_xp2p(host, "server", "service", "stop")


def deploy_client_to_server(
    client_host: Host,
    server_host: Host,
    *,
    client_log: PurePosixPath,
    server_log: PurePosixPath,
    server_ip: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
    log_wait_timeout: int,
) -> None:
    client_pid = None
    server_pid = None
    try:
        client_pid = cross_linux.start_linux_client_deploy(
            client_host,
            log_path=client_log,
            remote_host=server_ip,
            deploy_port=deploy_port,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=trojan_port,
        )
        link = cross_linux.wait_for_client_link_linux(
            client_host,
            client_log,
            timeout=log_wait_timeout,
        )
        server_pid = cross_linux.start_linux_server_deploy(
            server_host,
            log_path=server_log,
            listen_addr=f":{deploy_port}",
            deploy_link=link,
        )
        try:
            wait_for_any_log_phrase(
                server_host,
                server_log,
                [
                    "server deploy: apply request written",
                    "server deploy: completion requested",
                    "server deploy: starting xray-core",
                ],
                timeout=log_wait_timeout,
            )
            wait_for_any_log_phrase(
                client_host,
                client_log,
                [
                    "client deploy: completed",
                    "client deploy: client run active",
                ],
                timeout=log_wait_timeout,
            )
        except BaseException as exc:
            if not isinstance(exc, OutcomeException):
                raise
            debug = _collect_deploy_debug(
                client_host,
                server_host,
                client_log=client_log,
                server_log=server_log,
            )
            raise AssertionError(f"{exc}\n{debug}") from exc
    finally:
        if client_pid:
            linux_env.stop_process(client_host, str(client_pid))
        if server_pid:
            linux_env.stop_process(server_host, str(server_pid))


def wait_for_any_log_phrase(
    host: Host,
    path: PurePosixPath,
    phrases: list[str],
    *,
    timeout: int,
) -> str:
    expected_variants = [(phrase, f"xp2p: {phrase}") for phrase in phrases]

    def _matcher(text: str) -> str | None:
        for phrase, prefixed in expected_variants:
            if phrase in text or prefixed in text:
                return phrase
        return None

    return cross_linux.wait_for_log_value_linux(
        host,
        path,
        extractor=_matcher,
        description=f"any of {phrases} in {path}",
        timeout=timeout,
    )


def wait_for_apply_request_clear(host: Host, *, timeout_seconds: float) -> None:
    apply_path = helpers.CONFIG_ROOT / helpers.APPLY_DIR_NAME / "apply.request"
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if not linux_env.path_exists(host, apply_path):
            return
        time.sleep(1.0)
    pytest.fail(f"apply.request not cleared within {timeout_seconds:.0f}s at {apply_path}")


def assert_ping_ok(runner, target: str) -> None:
    result = runner(
        "ping",
        target,
        "-T",
        "--count",
        "3",
        check=True,
    )
    output = (result.stdout or "") + (result.stderr or "")
    if "0% loss" not in output.lower():
        raise AssertionError(f"xp2p ping did not report zero loss.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")


def assert_client_endpoints(host: Host, expected_hosts: set[str]) -> None:
    state = helpers.read_client_config(host)
    endpoints = state.get("endpoints", []) or []
    recorded_hosts = {entry.get("hostname") for entry in endpoints if entry.get("hostname")}
    if recorded_hosts != expected_hosts:
        raise AssertionError(
            "Unexpected client endpoints recorded.\n"
            f"Recorded: {recorded_hosts}\nExpected: {expected_hosts}\n"
            f"Config:\n{helpers.read_text(host, helpers.CLIENT_CONFIG_FILE)}"
        )


def assert_service_active(runner, role: str) -> None:
    result = runner(role, "service", "status")
    if result.rc != 0:
        raise AssertionError(
            f"xp2p {role} service is not active.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def log_offset(host: Host, path: PurePosixPath) -> int:
    if not helpers.path_exists(host, path):
        return 0
    return len((helpers.read_text(host, path) or "").splitlines())


def assert_no_service_stop(host: Host, path: PurePosixPath, offset: int, label: str) -> None:
    if not helpers.path_exists(host, path):
        pytest.fail(f"{label} service log missing at {path}")
    content = helpers.read_text(host, path)
    lines = content.splitlines()
    recent = "\n".join(lines[offset:])
    lowered = recent.lower()
    for marker in ("service stopped cleanly", "service run failed", "exceeded restart limit"):
        if marker in lowered:
            raise AssertionError(f"{label} service log reported {marker} after deploys.\nLog tail:\n{recent}")


def assert_server_state_reports_users(
    host: Host,
    expected_users: set[str],
    *,
    timeout_seconds: float = 60.0,
    poll_interval: float = 3.0,
) -> None:
    xp2p_binary = linux_env.INSTALL_PATH.as_posix()
    install_path = helpers.INSTALL_ROOT.as_posix()
    expected = {user.strip().lower() for user in expected_users if user.strip()}
    if not expected:
        pytest.fail("expected_users is empty")
    deadline = time.time() + timeout_seconds
    last_stdout = ""
    while time.time() < deadline:
        result = host.run(f"{xp2p_binary} server state --path {install_path}")
        if result.rc != 0:
            pytest.fail(
                "xp2p server state --once failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        last_stdout = result.stdout or ""
        users = {user.strip().lower() for user in _extract_client_users(last_stdout)}
        if expected.issubset(users):
            return
        time.sleep(poll_interval)
    pytest.fail(
        "xp2p server state did not report all expected users "
        f"{sorted(expected)} before timeout.\nLast output:\n{last_stdout}"
    )


def _collect_deploy_debug(
    client_host: Host,
    server_host: Host,
    *,
    client_log: PurePosixPath,
    server_log: PurePosixPath,
) -> str:
    client_details = _collect_host_debug(client_host, "client", deploy_log=client_log)
    server_details = _collect_host_debug(server_host, "server", deploy_log=server_log)
    return "\n\n".join([client_details, server_details])


def _collect_host_debug(host: Host, label: str, *, deploy_log: PurePosixPath) -> str:
    parts: list[str] = [f"{label} host debug:"]
    parts.append(_collect_log_tail(host, deploy_log, 200))
    service_log = helpers.LOG_ROOT / label / "service.log"
    if helpers.path_exists(host, service_log):
        parts.append(_collect_log_tail(host, service_log, 200))
    parts.append(_run_cmd(host, f"sudo -n ls -la /var/log/xp2p/{label} 2>/dev/null || true"))
    parts.append(_run_cmd(host, f"sudo -n /bin/sh -c 'for f in /var/log/xp2p/{label}/*.log; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; tail -n 200 \"$f\"; done'"))
    parts.append(_run_cmd(host, "sudo -n ss -lntp | sed -n '1,120p'"))
    parts.append(_run_cmd(host, "ps aux | grep -E 'xp2p|xray' | grep -v grep || true"))
    parts.append(_run_cmd(host, f"sudo -n {linux_env.INSTALL_PATH.as_posix()} {label} service status || true"))
    parts.append(_run_cmd(host, "sudo -n cat /etc/xp2p/.apply/apply.request 2>/dev/null || true"))
    parts.append(_run_cmd(host, "sudo -n ls -la /etc/xp2p/.apply /etc/xp2p/.apply/pending 2>/dev/null || true"))
    parts.append(_run_cmd(host, "sudo -n /bin/sh -c 'for f in /etc/xp2p/.apply/pending/*.toml; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done'"))
    parts.append(_run_cmd(host, "sudo -n ls -la /etc/xp2p/config-client /etc/xp2p/config-server 2>/dev/null || true"))
    parts.append(_run_cmd(host, f"sudo -n /bin/sh -c 'for f in /etc/xp2p/config-{label}/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done'"))
    parts.append(_run_cmd(host, f"sudo -n /bin/sh -c 'for f in /etc/xp2p/config-{label}/.apply/pending/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done'"))
    return "\n".join(parts)


def _collect_log_tail(host: Host, path: PurePosixPath, lines: int) -> str:
    command = f"sudo -n tail -n {int(lines)} {shlex.quote(path.as_posix())} 2>/dev/null || true"
    output = host.run(command).stdout or ""
    return f"{path}:\n{output}"


def _run_cmd(host: Host, command: str) -> str:
    result = host.run(command)
    stdout = result.stdout or ""
    stderr = result.stderr or ""
    return f"$ {command}\nrc={result.rc}\n{stdout}{stderr}"


def _extract_client_users(output: str) -> set[str]:
    cleaned = _strip_ansi(output)
    users: set[str] = set()
    for raw_line in cleaned.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("TAG"):
            continue
        if not line.startswith("proxy-"):
            continue
        columns = [segment.strip() for segment in re.split(r"\s{2,}", line) if segment.strip()]
        if len(columns) >= 7:
            users.add(columns[6])
    return users


def _strip_ansi(value: str | None) -> str:
    if not value:
        return ""
    return ANSI_ESCAPE_RE.sub("", value)
