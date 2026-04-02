from __future__ import annotations

import json
import shlex
import time
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.openwrt import env as openwrt_env

INSTALL_ROOT = linux_helpers.INSTALL_ROOT
CONFIG_ROOT = linux_helpers.CONFIG_ROOT
CLIENT_CONFIG_DIR_NAME = linux_helpers.CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR_NAME = linux_helpers.SERVER_CONFIG_DIR_NAME
CLIENT_CONFIG_DIR = linux_helpers.CLIENT_CONFIG_DIR
SERVER_CONFIG_DIR = linux_helpers.SERVER_CONFIG_DIR
APPLY_DIR_NAME = linux_helpers.APPLY_DIR_NAME
PENDING_DIR_NAME = linux_helpers.PENDING_DIR_NAME
CONFIG_PENDING_ROOT = CONFIG_ROOT / APPLY_DIR_NAME / PENDING_DIR_NAME
CLIENT_PENDING_DIR = CLIENT_CONFIG_DIR / APPLY_DIR_NAME / PENDING_DIR_NAME
SERVER_PENDING_DIR = SERVER_CONFIG_DIR / APPLY_DIR_NAME / PENDING_DIR_NAME
CLIENT_CONFIG_FILE = linux_helpers.CLIENT_CONFIG_FILE
SERVER_CONFIG_FILE = linux_helpers.SERVER_CONFIG_FILE
CLIENT_APPLIED_STATE_FILE = linux_helpers.CLIENT_APPLIED_STATE_FILE
SERVER_APPLIED_STATE_FILE = linux_helpers.SERVER_APPLIED_STATE_FILE
CLIENT_STATE_FILES = linux_helpers.CLIENT_STATE_FILES
SERVER_STATE_FILES = linux_helpers.SERVER_STATE_FILES
CLIENT_HEARTBEAT_STATE_FILE = linux_helpers.CLIENT_HEARTBEAT_STATE_FILE
SERVER_HEARTBEAT_STATE_FILE = linux_helpers.SERVER_HEARTBEAT_STATE_FILE
HEARTBEAT_STATE_FILE = linux_helpers.HEARTBEAT_STATE_FILE
CLIENT_LOG_FILE = linux_helpers.CLIENT_LOG_FILE
SERVER_LOG_FILE = linux_helpers.SERVER_LOG_FILE
LOG_ROOT = linux_helpers.LOG_ROOT
REVERSE_SUFFIX = linux_helpers.REVERSE_SUFFIX
XRAY_BINARY = linux_helpers.XRAY_BINARY
SERVICE_LOG_FILES = linux_helpers.SERVICE_LOG_FILES


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


def cleanup_client_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    print(f"==== cleanup client on {host.backend.hostname} ====")
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or CLIENT_CONFIG_DIR_NAME
    print(f"client remove start: {install_path} ({config_name})")
    runner(
        "client",
        "remove",
        "--path",
        install_path,
        "--config-dir",
        config_name,
        "--all",
        "--ignore-missing",
        "--quiet",
    )
    print("client remove done")
    remove_path(host, LOG_ROOT)
    print(f"client log root cleared: {LOG_ROOT.as_posix()}")
    openwrt_env.run_guest_script(host, "scripts/linux/ensure_dir.sh", LOG_ROOT.as_posix(), "0777")
    print("==== cleanup client done ====")


def cleanup_server_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    print(f"==== cleanup server on {host.backend.hostname} ====")
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or SERVER_CONFIG_DIR_NAME
    print(f"server remove start: {install_path} ({config_name})")
    runner(
        "server",
        "remove",
        "--path",
        install_path,
        "--config-dir",
        config_name,
        "--ignore-missing",
        "--quiet",
    )
    print("server remove done")
    remove_path(host, LOG_ROOT)
    print(f"server log root cleared: {LOG_ROOT.as_posix()}")
    openwrt_env.run_guest_script(host, "scripts/linux/ensure_dir.sh", LOG_ROOT.as_posix(), "0777")
    print("==== cleanup server done ====")


def find_tun_inbound(data: dict) -> dict | None:
    for inbound in data.get("inbounds", []) or []:
        if isinstance(inbound, dict) and inbound.get("protocol") == "tun":
            return inbound
    return None


def assert_tun_inbound(data: dict, expected_name: str) -> None:
    inbound = find_tun_inbound(data)
    if not inbound:
        raise AssertionError("Expected tun inbound not found")
    settings = inbound.get("settings") or {}
    recorded_name = (settings.get("name") or "").strip()
    if recorded_name != expected_name:
        raise AssertionError(f"Expected tun inbound name {expected_name}, got {recorded_name}")


def assert_no_tun_inbound(data: dict) -> None:
    inbound = find_tun_inbound(data)
    if inbound:
        raise AssertionError(f"Unexpected tun inbound present: {inbound}")


def _posix(value: PurePosixPath | Path | str) -> str:
    if isinstance(value, (PurePosixPath, Path)):
        return value.as_posix()
    return str(value)


def file_sha256(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(path)
    result = host.run(f"sha256sum {shlex.quote(target)}")
    if result.rc != 0 or not result.stdout:
        raise RuntimeError(
            f"Failed to hash remote file {target}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return (result.stdout.strip().split()[0]).strip()


def read_text(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(_resolve_config_path(host, _as_path(path)))
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


def read_toml(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_text(host, path)
    try:
        return linux_helpers.tomllib.loads(content)
    except linux_helpers.tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_first_existing_json(host: Host, paths: list[PurePosixPath]) -> dict:
    for candidate in paths:
        if path_exists(host, candidate):
            return read_json(host, candidate)
    raise AssertionError(f"None of the state files exist: {paths}")


def path_exists(host: Host, path: PurePosixPath | Path | str) -> bool:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    if pending != resolved and linux_helpers.path_exists(host, pending):
        return True
    target = _posix(resolved)
    result = host.run(f"test -e {shlex.quote(target)}")
    return result.rc == 0


def remove_path(host: Host, path: PurePosixPath | Path | str) -> None:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    target = _posix(pending)
    host.run(f"rm -rf {shlex.quote(target)} >/dev/null 2>&1 || true")
    if pending != resolved:
        target = _posix(resolved)
        host.run(f"rm -rf {shlex.quote(target)} >/dev/null 2>&1 || true")


def read_client_config(host: Host) -> dict:
    return read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}


def read_server_config(host: Host) -> dict:
    return read_toml(host, SERVER_CONFIG_FILE).get("server") or {}


def dump_install_dirs(host: Host, label: str) -> None:
    paths = [INSTALL_ROOT, CLIENT_CONFIG_DIR, SERVER_CONFIG_DIR]
    print(f"==== INSTALL DIRS ({label}) on {host.backend.hostname} ====")
    for path in paths:
        target = path.as_posix()
        exists = host.run(f"test -d {shlex.quote(target)}").rc == 0
        status = "present" if exists else "missing"
        print(f"{target}: {status}")
        if exists:
            listing = host.run(f"ls -lha {shlex.quote(target)}")
            if listing.stdout:
                print(listing.stdout)
    print("==== END INSTALL DIRS ====")


def write_text(host: Host, path: PurePosixPath | Path | str, content: str) -> None:
    target = _posix(_pending_candidate(_as_path(path)))
    directory = PurePosixPath(target).parent.as_posix()
    host.run(f"mkdir -p {shlex.quote(directory)}")
    marker = "XP2P_EOF"
    command = (
        f"cat <<'{marker}' > {shlex.quote(target)}\n"
        f"{content}\n"
        f"{marker}\n"
    )
    result = host.run(command)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {target}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def write_apply_request(host: Host, role: str) -> None:
    payload = f'{{"role":"{role}"}}\\n'
    path = CONFIG_ROOT / APPLY_DIR_NAME / "apply.request"
    write_text(host, path, payload)


def dump_logs(host: Host, label: str, paths: list[PurePosixPath] | None = None, *, tail_lines: int = 200) -> None:
    entries = paths or [CLIENT_LOG_FILE, SERVER_LOG_FILE, *SERVICE_LOG_FILES]
    header = f"==== XP2P LOGS ({label}) on {host.backend.hostname} ===="
    print(header)
    for path in entries:
        if not path_exists(host, path):
            print(f"-- {path} (missing)")
            continue
        content = read_text(host, path)
        lines = content.splitlines()
        tail = "\n".join(lines[-tail_lines:])
        print(f"-- {path} (tail {min(len(lines), tail_lines)} lines)")
        print(tail)
    print("=" * len(header))


def _pending_candidate(path: PurePosixPath) -> PurePosixPath:
    if path.is_relative_to(CONFIG_ROOT / APPLY_DIR_NAME):
        return path
    if path.is_relative_to(CLIENT_CONFIG_DIR / APPLY_DIR_NAME):
        return path
    if path.is_relative_to(SERVER_CONFIG_DIR / APPLY_DIR_NAME):
        return path
    if path.is_relative_to(INSTALL_ROOT):
        relative = path.relative_to(INSTALL_ROOT)
        if relative.parts:
            config_root = relative.parts[0]
            if config_root.startswith("config-"):
                return INSTALL_ROOT / config_root / APPLY_DIR_NAME / PENDING_DIR_NAME / PurePosixPath(*relative.parts[1:])
    if path.is_relative_to(CLIENT_CONFIG_DIR):
        return CLIENT_PENDING_DIR / path.relative_to(CLIENT_CONFIG_DIR)
    if path.is_relative_to(SERVER_CONFIG_DIR):
        return SERVER_PENDING_DIR / path.relative_to(SERVER_CONFIG_DIR)
    if path.is_relative_to(CONFIG_ROOT):
        return CONFIG_PENDING_ROOT / path.relative_to(CONFIG_ROOT)
    return path


def _resolve_config_path(host: Host, path: PurePosixPath) -> PurePosixPath:
    pending = _pending_candidate(path)
    if pending != path and linux_helpers.path_exists(host, pending):
        return pending
    return path


def _as_path(path: PurePosixPath | Path | str) -> PurePosixPath:
    if isinstance(path, PurePosixPath):
        return path
    if isinstance(path, Path):
        return PurePosixPath(path.as_posix())
    return PurePosixPath(str(path))


def read_client_applied_state(host: Host) -> dict:
    return read_json(host, CLIENT_APPLIED_STATE_FILE)


def read_server_applied_state(host: Host) -> dict:
    return read_json(host, SERVER_APPLIED_STATE_FILE)


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
