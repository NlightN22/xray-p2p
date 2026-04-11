from __future__ import annotations

import json
import shlex
import time
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.linux import env as linux_env
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
APPLY_REQUEST = CONFIG_ROOT / APPLY_DIR_NAME / "apply.request"


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


def assert_reverse_cli_output_live(
    runner,
    role: str,
    install_dir: PurePosixPath | str,
    config_dir: str,
    reverse_tag: str,
) -> None:
    install_path = install_dir.as_posix() if isinstance(install_dir, PurePosixPath) else str(install_dir)
    result = runner(
        role,
        "reverse",
        "--path",
        install_path,
        "--config-dir",
        config_dir,
        check=True,
    )
    output = (result.stdout or "").lower()
    tag = reverse_tag.strip().lower()
    assert tag in output, f"{role} reverse list output missing {reverse_tag}. STDOUT: {result.stdout}"


def cleanup_client_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    print(f"==== cleanup client on {host.backend.hostname} ====")
    cleanup_runtime_artifacts(host)
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or CLIENT_CONFIG_DIR_NAME
    print(f"client cleanup start: {install_path} ({config_name})")
    _purge_install_paths(host, install_path, config_name, "client")
    print("client cleanup done")
    _clear_log_root(host)
    print("==== cleanup client done ====")


def cleanup_server_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    print(f"==== cleanup server on {host.backend.hostname} ====")
    cleanup_runtime_artifacts(host)
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or SERVER_CONFIG_DIR_NAME
    print(f"server cleanup start: {install_path} ({config_name})")
    _purge_install_paths(host, install_path, config_name, "server")
    print("server cleanup done")
    _clear_log_root(host)
    print("==== cleanup server done ====")


def _clear_log_root(host: Host) -> None:
    target = shlex.quote(LOG_ROOT.as_posix())
    host.run(
        "/bin/sh -c "
        "'if [ -d \"$1\" ]; then "
        "rm -rf \"$1\"/* \"$1\"/.[!.]* \"$1\"/..?* >/dev/null 2>&1 || true; "
        "fi' "
        f"-- {target}"
    )
    print(f"log root cleared: {LOG_ROOT.as_posix()}")


def cleanup_runtime_artifacts(host: Host) -> None:
    openwrt_env._stop_xp2p_services(host)
    host.run("/etc/init.d/xp2p-client disable >/dev/null 2>&1 || true")
    host.run("/etc/init.d/xp2p-server disable >/dev/null 2>&1 || true")
    openwrt_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
    for port in ("62022", "62023", "52080", "52180", "51080", "51180"):
        host.run(f"fuser -k {port}/tcp >/dev/null 2>&1 || true")
        host.run(f"fuser -k {port}/udp >/dev/null 2>&1 || true")
        openwrt_env._kill_port_listeners(host, port)
    host.run("rm -f /tmp/xp2p-*.log >/dev/null 2>&1 || true")
    dump_runtime_state(host, "after cleanup")


def _purge_install_paths(host: Host, install_path: str, config_name: str, role: str) -> None:
    if role not in {"client", "server"}:
        raise ValueError(f"Unsupported role: {role}")
    config_path = f"{install_path.rstrip('/')}/{config_name}"
    pending_root = CONFIG_PENDING_ROOT.as_posix()
    pending_config = f"{pending_root}/xp2p-{role}.toml"
    pending_heartbeat = f"{pending_root}/state-heartbeat-{role}.json"
    state_files = CLIENT_STATE_FILES if role == "client" else SERVER_STATE_FILES
    heartbeat = CLIENT_HEARTBEAT_STATE_FILE if role == "client" else SERVER_HEARTBEAT_STATE_FILE
    targets = [
        config_path,
        pending_config,
        pending_heartbeat,
        APPLY_REQUEST.as_posix(),
        heartbeat.as_posix(),
    ]
    for item in state_files:
        targets.append(item.as_posix())
    cmd = "/bin/sh -c 'rm -rf -- \"$@\"' -- " + " ".join(shlex.quote(p) for p in targets)
    host.run(cmd)


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


def read_live_text(host: Host, path: PurePosixPath | Path | str) -> str:
    return linux_env.read_text(host, _as_path(path))


def read_live_json(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_live_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def read_live_toml(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_live_text(host, path)
    try:
        return linux_helpers.tomllib.loads(content)
    except linux_helpers.tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_live_client_config(host: Host) -> dict:
    config = CONFIG_ROOT / "xp2p-client.toml"
    return read_live_toml(host, config).get("client") or {}


def read_live_server_config(host: Host) -> dict:
    config = CONFIG_ROOT / "xp2p-server.toml"
    return read_live_toml(host, config).get("server") or {}


def read_first_existing_json(host: Host, paths: list[PurePosixPath]) -> dict:
    for candidate in paths:
        if path_exists(host, candidate):
            return read_json(host, candidate)
    raise AssertionError(f"None of the state files exist: {paths}")


def pending_path(path: PurePosixPath | Path | str) -> PurePosixPath:
    return _pending_candidate(_as_path(path))


def read_pending_text(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(_pending_candidate(_as_path(path)))
    result = host.run(f"cat {shlex.quote(target)}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {target}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def read_pending_json(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_pending_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def read_pending_toml(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_pending_text(host, path)
    try:
        return linux_helpers.tomllib.loads(content)
    except linux_helpers.tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_pending_client_config(host: Host) -> dict:
    pending_config = CONFIG_PENDING_ROOT / "xp2p-client.toml"
    return read_pending_toml(host, pending_config).get("client") or {}


def read_pending_server_config(host: Host) -> dict:
    pending_config = CONFIG_PENDING_ROOT / "xp2p-server.toml"
    return read_pending_toml(host, pending_config).get("server") or {}


def read_preferred_text(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _as_path(path)
    if path_exists_live(host, target):
        return read_live_text(host, target)
    return read_text(host, target)


def read_preferred_json(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_preferred_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def read_preferred_toml(host: Host, path: PurePosixPath | Path | str) -> dict:
    content = read_preferred_text(host, path)
    try:
        return linux_helpers.tomllib.loads(content)
    except linux_helpers.tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_preferred_client_config(host: Host) -> dict:
    config = CONFIG_ROOT / "xp2p-client.toml"
    return read_preferred_toml(host, config).get("client") or {}


def read_preferred_server_config(host: Host) -> dict:
    config = CONFIG_ROOT / "xp2p-server.toml"
    return read_preferred_toml(host, config).get("server") or {}


def path_exists(host: Host, path: PurePosixPath | Path | str) -> bool:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    if pending != resolved and linux_helpers.path_exists(host, pending):
        return True
    target = _posix(resolved)
    result = host.run(f"test -e {shlex.quote(target)}")
    return result.rc == 0


def path_exists_exact(host: Host, path: PurePosixPath | Path | str) -> bool:
    return linux_helpers.path_exists(host, _as_path(path))


def path_exists_live(host: Host, path: PurePosixPath | Path | str) -> bool:
    target = _posix(_as_path(path))
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


def dump_runtime_state(host: Host, label: str) -> None:
    print(f"==== RUNTIME STATE ({label}) on {host.backend.hostname} ====")
    host.run(
        "sh -c "
        + shlex.quote(
            "echo '-- ps xp2p/xray'; "
            "ps w | grep -E 'xp2p|xray' | grep -v grep || echo 'no xp2p/xray processes'; "
            "echo '-- netstat listeners'; "
            "netstat -lpn 2>/dev/null | egrep '62022|62023|52080|52180|51080|51180' || echo 'no xp2p listeners'; "
            "echo '-- fuser ports'; "
            "fuser -n tcp 62022 62023 52080 52180 51080 51180 2>/dev/null || echo 'no tcp fuser hits'; "
            "fuser -n udp 62022 62023 52080 52180 51080 51180 2>/dev/null || echo 'no udp fuser hits'"
        )
    )
    print("==== END RUNTIME STATE ====")

def dump_apply_dirs(host: Host, label: str) -> None:
    dirs = [
        CONFIG_ROOT / APPLY_DIR_NAME,
        CONFIG_PENDING_ROOT,
        CLIENT_CONFIG_DIR / APPLY_DIR_NAME,
        CLIENT_PENDING_DIR,
        SERVER_CONFIG_DIR / APPLY_DIR_NAME,
        SERVER_PENDING_DIR,
    ]
    files = [
        CONFIG_ROOT / "xp2p-client.toml",
        CONFIG_ROOT / "xp2p-server.toml",
        CONFIG_PENDING_ROOT / "xp2p-client.toml",
        CONFIG_PENDING_ROOT / "xp2p-server.toml",
        APPLY_REQUEST,
    ]
    print(f"==== APPLY DIRS ({label}) on {host.backend.hostname} ====")
    for path in dirs:
        target = path.as_posix()
        exists = host.run(f"test -d {shlex.quote(target)}").rc == 0
        status = "present" if exists else "missing"
        print(f"{target}: {status}")
        if exists:
            listing = host.run(f"ls -lha {shlex.quote(target)}")
            if listing.stdout:
                print(listing.stdout)
    for path in files:
        target = path.as_posix()
        exists = host.run(f"test -e {shlex.quote(target)}").rc == 0
        status = "present" if exists else "missing"
        print(f"{target}: {status}")
    print("==== END APPLY DIRS ====")


def write_text(host: Host, path: PurePosixPath | Path | str, content: str) -> None:
    target = _posix(_pending_candidate(_as_path(path)))
    directory = PurePosixPath(target).parent.as_posix()
    check = host.run(f"test -d {shlex.quote(directory)}")
    if check.rc != 0:
        raise RuntimeError(f"Parent directory does not exist: {directory}")
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


def wait_for_apply_request(host: Host, *, timeout_seconds: float = 30.0, poll_interval: float = 1.5) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if path_exists_exact(host, APPLY_REQUEST):
            return
        time.sleep(poll_interval)
    raise AssertionError(f"apply.request did not appear within {timeout_seconds} seconds.")


def wait_for_apply_request_clear(
    host: Host, *, timeout_seconds: float = 60.0, poll_interval: float = 1.5
) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if not path_exists_exact(host, APPLY_REQUEST):
            return
        time.sleep(poll_interval)
    raise AssertionError(f"apply.request did not clear after {timeout_seconds} seconds.")


def wait_for_pending_config(host: Host, role: str, *, timeout_seconds: float = 30.0, poll_interval: float = 1.5) -> None:
    if role == "client":
        target = CONFIG_PENDING_ROOT / "xp2p-client.toml"
    elif role == "server":
        target = CONFIG_PENDING_ROOT / "xp2p-server.toml"
    else:
        raise ValueError(f"Unsupported role: {role}")
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if path_exists_exact(host, target):
            return
        time.sleep(poll_interval)
    raise AssertionError(f"Pending config {target} did not appear within {timeout_seconds} seconds.")


def wait_for_live_config(
    host: Host,
    role: str,
    *,
    timeout_seconds: float = 30.0,
    poll_interval: float = 1.5,
) -> None:
    if role == "client":
        target = CONFIG_ROOT / "xp2p-client.toml"
    elif role == "server":
        target = CONFIG_ROOT / "xp2p-server.toml"
    else:
        raise ValueError(f"Unsupported role: {role}")
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if path_exists_exact(host, target):
            return
        time.sleep(poll_interval)
    raise AssertionError(f"Live config {target} did not appear within {timeout_seconds} seconds.")


def is_xp2p_run_active(host: Host, role: str) -> bool:
    cmd = (
        "ps w | "
        "grep -E "
        + shlex.quote(rf"xp2p {role} (run|service run)")
        + " | grep -v grep >/dev/null 2>&1"
    )
    return host.run(cmd).rc == 0


def wait_for_service_state(
    host: Host,
    role: str,
    expected_active: bool,
    *,
    timeout_seconds: float = 45.0,
    poll_interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(poll_interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def ensure_service_running(host: Host, role: str) -> None:
    if is_xp2p_run_active(host, role):
        return
    start = host.run(f"/etc/init.d/xp2p-{role} start")
    if start.rc != 0:
        raise AssertionError(
            "Failed to start service "
            f"xp2p-{role} on {host.backend.hostname}.\nSTDOUT:\n{start.stdout}\nSTDERR:\n{start.stderr}"
        )
    wait_for_service_state(host, role, expected_active=True)
