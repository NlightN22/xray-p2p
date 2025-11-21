from __future__ import annotations

import json
import shlex
import time
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers

INSTALL_ROOT = linux_helpers.INSTALL_ROOT
CLIENT_CONFIG_DIR_NAME = linux_helpers.CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR_NAME = linux_helpers.SERVER_CONFIG_DIR_NAME
CLIENT_CONFIG_DIR = linux_helpers.CLIENT_CONFIG_DIR
SERVER_CONFIG_DIR = linux_helpers.SERVER_CONFIG_DIR
CLIENT_STATE_FILES = linux_helpers.CLIENT_STATE_FILES
SERVER_STATE_FILES = linux_helpers.SERVER_STATE_FILES
HEARTBEAT_STATE_FILE = linux_helpers.HEARTBEAT_STATE_FILE
CLIENT_LOG_FILE = linux_helpers.CLIENT_LOG_FILE
SERVER_LOG_FILE = linux_helpers.SERVER_LOG_FILE
REVERSE_SUFFIX = linux_helpers.REVERSE_SUFFIX

cleanup_client_install = linux_helpers.cleanup_client_install
cleanup_server_install = linux_helpers.cleanup_server_install
extract_trojan_credential = linux_helpers.extract_trojan_credential
expected_proxy_tag = linux_helpers.expected_proxy_tag
expected_reverse_tag = linux_helpers.expected_reverse_tag
assert_routing_rule = linux_helpers.assert_routing_rule
assert_heartbeat_entry = linux_helpers.assert_heartbeat_entry
detect_primary_ipv4 = linux_helpers.detect_primary_ipv4
assert_reverse_cli_output = linux_helpers.assert_reverse_cli_output
assert_client_reverse_artifacts = linux_helpers.assert_client_reverse_artifacts
assert_client_reverse_state = linux_helpers.assert_client_reverse_state
assert_server_reverse_state = linux_helpers.assert_server_reverse_state
assert_server_reverse_routing = linux_helpers.assert_server_reverse_routing
assert_server_redirect_state = linux_helpers.assert_server_redirect_state
assert_server_redirect_rule = linux_helpers.assert_server_redirect_rule
assert_redirect_rule = linux_helpers.assert_redirect_rule
assert_no_redirect_rule = linux_helpers.assert_no_redirect_rule
assert_outbound = linux_helpers.assert_outbound


def _posix(value: PurePosixPath | Path | str) -> str:
    if isinstance(value, (PurePosixPath, Path)):
        return value.as_posix()
    return str(value)


def read_text(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(path)
    result = host.run(f"cat {shlex.quote(target)}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {target}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def read_json(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def read_first_existing_json(host: Host, paths: list[PurePosixPath]) -> dict:
    for candidate in paths:
        if path_exists(host, candidate):
            return read_json(host, candidate)
    raise AssertionError(f"None of the state files exist: {paths}")


def path_exists(host: Host, path: PurePosixPath | Path | str) -> bool:
    target = _posix(path)
    result = host.run(f"test -e {shlex.quote(target)}")
    return result.rc == 0


def remove_path(host: Host, path: PurePosixPath | Path | str) -> None:
    target = _posix(path)
    host.run(f"rm -rf {shlex.quote(target)} >/dev/null 2>&1 || true")


def wait_for_heartbeat_state(
    host: Host,
    path: PurePosixPath | None = None,
    *,
    timeout_seconds: float = 60.0,
    poll_interval: float = 1.5,
) -> dict:
    target = path or HEARTBEAT_STATE_FILE
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        if path_exists(host, target):
            try:
                return read_json(host, target)
            except RuntimeError as exc:
                last_error = exc
        time.sleep(poll_interval)
    if last_error:
        raise AssertionError(f"Failed to read heartbeat state {target}: {last_error}") from last_error
    raise AssertionError(f"Heartbeat state {target} not found on {host.backend.hostname}")
