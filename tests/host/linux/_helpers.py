from __future__ import annotations

import json
import os
import shlex
import sys
from pathlib import PurePosixPath
import time
import uuid
from datetime import datetime, timezone

from testinfra.host import Host

from tests.host.linux import env as linux_env

try:
    import tomllib
except ImportError:  # pragma: no cover - fallback for older runtimes.
    import tomli as tomllib

INSTALL_ROOT = PurePosixPath("/etc/xp2p")
CONFIG_ROOT = PurePosixPath(os.environ.get("XP2P_CONFIG_ROOT", "/etc/xp2p"))
CLIENT_CONFIG_DIR_NAME = "config-client"
SERVER_CONFIG_DIR_NAME = "config-server"
CLIENT_CONFIG_DIR = INSTALL_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR = INSTALL_ROOT / SERVER_CONFIG_DIR_NAME
APPLY_DIR_NAME = ".state"
PENDING_DIR_NAME = "pending"
LIVE_DIR_NAME = "live"
LKG_DIR_NAME = "lkg"
STATE_ROOT = CONFIG_ROOT / APPLY_DIR_NAME
CONFIG_PENDING_ROOT = STATE_ROOT / PENDING_DIR_NAME
CONFIG_LIVE_ROOT = STATE_ROOT / LIVE_DIR_NAME
CONFIG_LKG_ROOT = STATE_ROOT / LKG_DIR_NAME
CLIENT_PENDING_DIR = CONFIG_PENDING_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_PENDING_DIR = CONFIG_PENDING_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_LIVE_DIR = CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_LIVE_DIR = CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_CONFIG_FILE = CONFIG_ROOT / "xp2p-client.toml"
SERVER_CONFIG_FILE = CONFIG_ROOT / "xp2p-server.toml"
CLIENT_APPLIED_STATE_FILE = CONFIG_ROOT / "xp2p-client.state.json"
SERVER_APPLIED_STATE_FILE = CONFIG_ROOT / "xp2p-server.state.json"
CLIENT_STATE_FILES = [CLIENT_CONFIG_FILE, CLIENT_APPLIED_STATE_FILE]
SERVER_STATE_FILES = [SERVER_CONFIG_FILE, SERVER_APPLIED_STATE_FILE]
CLIENT_HEARTBEAT_STATE_FILE = INSTALL_ROOT / "state-heartbeat-client.json"
SERVER_HEARTBEAT_STATE_FILE = INSTALL_ROOT / "state-heartbeat-server.json"
HEARTBEAT_STATE_FILE = CLIENT_HEARTBEAT_STATE_FILE
LOG_ROOT = PurePosixPath(os.environ.get("XP2P_LOG_ROOT", "/var/log/xp2p"))
CLIENT_LOG_FILE = PurePosixPath("/tmp/xp2p-client-run.log")
SERVER_LOG_FILE = PurePosixPath("/tmp/xp2p-server-run.log")
SERVICE_LOG_FILES = (
    LOG_ROOT / "client" / "service.log",
    LOG_ROOT / "server" / "service.log",
)
XRAY_BINARY = INSTALL_ROOT / "bin" / "xray"
REVERSE_SUFFIX = ".rev"


def safe_output(value: str | None) -> str:
    if not value:
        return ""
    return value.encode("ascii", "backslashreplace").decode("ascii")


def cleanup_client_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or CLIENT_CONFIG_DIR_NAME
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
    _cleanup_state(host)
    remove_log_files(host)


def cleanup_server_install(
    host: Host,
    runner,
    install_dir: PurePosixPath | None = None,
    config_dir: str | None = None,
) -> None:
    install_path = (install_dir or INSTALL_ROOT).as_posix()
    config_name = config_dir or SERVER_CONFIG_DIR_NAME
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
    _cleanup_state(host)
    remove_log_files(host)


def assert_reverse_cli_output(
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
    if tag not in output:
        result = runner(
            role,
            "reverse",
            "--path",
            install_path,
            "--config-dir",
            config_dir,
            "--pending",
            check=True,
        )
        output = (result.stdout or "").lower()
    assert tag in output, f"{role} reverse list output missing {reverse_tag}. STDOUT: {result.stdout}"


def read_json(host: Host, path: PurePosixPath | str) -> dict:
    return linux_env.read_json(host, _as_path(path))


def read_toml(host: Host, path: PurePosixPath) -> dict:
    content = read_text(host, path)
    try:
        return tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_first_existing_json(host: Host, paths: list[PurePosixPath]) -> dict:
    for path in paths:
        if linux_env.path_exists(host, path):
            return read_json(host, path)
    raise AssertionError(f"None of the state files exist: {paths}")


def read_text(host: Host, path: PurePosixPath | str) -> str:
    return linux_env.read_text(host, _as_path(path))


def pending_path(path: PurePosixPath | str) -> PurePosixPath:
    return _pending_candidate(_as_path(path))


def read_pending_json(host: Host, path: PurePosixPath | str) -> dict:
    return linux_env.read_json(host, pending_path(path))


def read_pending_text(host: Host, path: PurePosixPath | str) -> str:
    return linux_env.read_text(host, pending_path(path))


def read_pending_toml(host: Host, path: PurePosixPath) -> dict:
    content = read_pending_text(host, path)
    try:
        return tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def read_pending_client_config(host: Host) -> dict:
    pending_config = CONFIG_PENDING_ROOT / "xp2p-client.toml"
    return read_pending_toml(host, pending_config).get("client") or {}


def read_pending_server_config(host: Host) -> dict:
    pending_config = CONFIG_PENDING_ROOT / "xp2p-server.toml"
    return read_pending_toml(host, pending_config).get("server") or {}


def path_exists(host: Host, path: PurePosixPath | str) -> bool:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    if pending != resolved and linux_env.path_exists(host, pending):
        return True
    return linux_env.path_exists(host, resolved)


def remove_path(host: Host, path: PurePosixPath) -> None:
    resolved = _as_path(path)
    pending = _pending_candidate(resolved)
    linux_env.remove_path(host, pending)
    if pending != resolved:
        linux_env.remove_path(host, resolved)


def remove_log_files(host: Host) -> None:
    for path in (CLIENT_LOG_FILE, SERVER_LOG_FILE, *SERVICE_LOG_FILES):
        linux_env.remove_path(host, path)


def _cleanup_state(host: Host) -> None:
    cleanup_paths = [
        CONFIG_ROOT / ".apply",
        STATE_ROOT,
        CONFIG_PENDING_ROOT,
        CONFIG_LIVE_ROOT,
        CONFIG_LKG_ROOT,
        STATE_ROOT / "apply.request",
        STATE_ROOT / "apply.error",
        CONFIG_ROOT / "xp2p-client.toml",
        CONFIG_ROOT / "xp2p-server.toml",
        CONFIG_ROOT / "xp2p-client.toml.lkg",
        CONFIG_ROOT / "xp2p-server.toml.lkg",
        CONFIG_ROOT / "xp2p-client.state.json",
        CONFIG_ROOT / "xp2p-server.state.json",
        CONFIG_ROOT / "xp2p-client.state.json.lkg",
        CONFIG_ROOT / "xp2p-server.state.json.lkg",
        CONFIG_ROOT / "xp2p-client.tun-full.json",
        CONFIG_ROOT / "xp2p-server.tun-full.json",
        CLIENT_HEARTBEAT_STATE_FILE,
        SERVER_HEARTBEAT_STATE_FILE,
        CLIENT_CONFIG_DIR,
        SERVER_CONFIG_DIR,
        CLIENT_CONFIG_DIR / "inbounds.json.lkg",
        CLIENT_CONFIG_DIR / "outbounds.json.lkg",
        CLIENT_CONFIG_DIR / "routing.json.lkg",
        CLIENT_CONFIG_DIR / "logs.json.lkg",
        SERVER_CONFIG_DIR / "inbounds.json.lkg",
        SERVER_CONFIG_DIR / "outbounds.json.lkg",
        SERVER_CONFIG_DIR / "routing.json.lkg",
        SERVER_CONFIG_DIR / "logs.json.lkg",
        CLIENT_PENDING_DIR,
        SERVER_PENDING_DIR,
        CLIENT_LIVE_DIR,
        SERVER_LIVE_DIR,
    ]
    for path in cleanup_paths:
        linux_env.remove_path(host, path)


def write_text(host: Host, path: PurePosixPath | str, content: str) -> None:
    linux_env.write_text(host, _pending_candidate(_as_path(path)), content)


def file_sha256(host: Host, path: PurePosixPath | str) -> str:
    return linux_env.file_sha256(host, _resolve_config_path(host, _as_path(path)))


def write_apply_request(host: Host, role: str) -> None:
    payload = json.dumps(
        {
            "id": str(uuid.uuid4()),
            "timestamp": datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
            "role": role,
        }
    )
    payload = f"{payload}\n"
    path = CONFIG_ROOT / APPLY_DIR_NAME / "apply.request"
    linux_env.write_text(host, path, payload)


def _pending_candidate(path: PurePosixPath | str) -> PurePosixPath:
    path = _as_path(path)
    if path.is_relative_to(CONFIG_PENDING_ROOT):
        return path
    if path.is_relative_to(CONFIG_LIVE_ROOT):
        return CONFIG_PENDING_ROOT / path.relative_to(CONFIG_LIVE_ROOT)
    if path.is_relative_to(CONFIG_LKG_ROOT):
        return path
    if path.is_relative_to(CLIENT_CONFIG_DIR):
        return CLIENT_PENDING_DIR / path.relative_to(CLIENT_CONFIG_DIR)
    if path.is_relative_to(SERVER_CONFIG_DIR):
        return SERVER_PENDING_DIR / path.relative_to(SERVER_CONFIG_DIR)
    if path.is_relative_to(CONFIG_ROOT):
        return CONFIG_PENDING_ROOT / path.relative_to(CONFIG_ROOT)
    return path


def _resolve_config_path(host: Host, path: PurePosixPath) -> PurePosixPath:
    pending = _pending_candidate(path)
    if pending != path and linux_env.path_exists(host, pending):
        return pending
    return path


def _as_path(path: PurePosixPath | str) -> PurePosixPath:
    if isinstance(path, PurePosixPath):
        return path
    return PurePosixPath(str(path))


def read_client_config(host: Host) -> dict:
    live_path = CONFIG_LIVE_ROOT / "xp2p-client.toml"
    if linux_env.path_exists(host, live_path):
        return read_toml(host, live_path).get("client") or {}
    return read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}


def read_server_config(host: Host) -> dict:
    live_path = CONFIG_LIVE_ROOT / "xp2p-server.toml"
    if linux_env.path_exists(host, live_path):
        return read_toml(host, live_path).get("server") or {}
    return read_toml(host, SERVER_CONFIG_FILE).get("server") or {}


def read_client_applied_state(host: Host) -> dict:
    return read_json(host, CLIENT_APPLIED_STATE_FILE)


def read_server_applied_state(host: Host) -> dict:
    return read_json(host, SERVER_APPLIED_STATE_FILE)


def detect_primary_ipv4(host: Host) -> str:
    command = "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1 | head -n1"
    result = host.run(command)
    ip_address = (result.stdout or "").strip()
    if result.rc != 0 or not ip_address:
        raise AssertionError(
            f"Unable to detect primary IPv4 address on {host.backend.hostname}.\n"
            f"CMD: {command}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return ip_address


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
        if linux_env.path_exists(host, target):
            try:
                return read_json(host, target)
            except RuntimeError as exc:
                last_error = exc
        time.sleep(poll_interval)
    if last_error:
        raise AssertionError(f"Failed to read heartbeat state {target}: {last_error}") from last_error
    raise AssertionError(f"Heartbeat state {target} not found on {host.backend.hostname}")


def assert_heartbeat_entry(
    state: dict,
    tag: str,
    *,
    host: str | None = None,
    user: str | None = None,
    client_ip: str | None = None,
) -> dict:
    entries = state.get("entries")
    if not isinstance(entries, dict):
        raise AssertionError("Heartbeat state is missing entries map")
    normalized = (tag or "").strip().lower()
    if not normalized:
        raise AssertionError("Heartbeat tag to look up is empty")
    for entry in entries.values():
        entry_tag = (entry.get("tag") or "").strip()
        if entry_tag.lower() != normalized:
            continue
        if host is not None:
            recorded_host = (entry.get("host") or "").strip()
            if recorded_host != host.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} host mismatch (expected {host}, got {recorded_host})"
                )
        if user is not None:
            recorded_user = (entry.get("user") or "").strip()
            if recorded_user != user.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} user mismatch (expected {user}, got {recorded_user})"
                )
        if client_ip is not None:
            recorded_ip = (entry.get("client_ip") or "").strip()
            if recorded_ip != client_ip.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} client IP mismatch (expected {client_ip}, got {recorded_ip})"
                )
        return entry
    raise AssertionError(f"Heartbeat entry for tag {tag} not found in state")


def extract_trojan_credential(output: str) -> dict[str, str]:
    user = password = link = None
    for raw in (output or "").splitlines():
        line = raw.strip()
        lowered = line.lower()
        if lowered.startswith("user:"):
            user = line.split(":", 1)[1].strip()
        elif lowered.startswith("password:"):
            password = line.split(":", 1)[1].strip()
        elif lowered.startswith("link:"):
            link = line.split(":", 1)[1].strip()
    if not user or not password:
        raise RuntimeError(
            "xp2p server install did not emit trojan credential lines.\n"
            f"STDOUT:\n{output}"
        )
    if not link:
        raise RuntimeError(
            "xp2p server install did not emit trojan link.\n"
            f"STDOUT:\n{output}"
        )
    return {"user": user, "password": password, "link": link}


def expected_proxy_tag(host: str) -> str:
    cleaned = "".join(_sanitize_host(host)).strip("-")
    if not cleaned:
        cleaned = "endpoint"
    return f"proxy-{cleaned}"


def expected_reverse_tag(user: str, host: str) -> str:
    user_label = _sanitize_label(user)
    host_label = _sanitize_label(host)
    if not user_label or not host_label:
        raise AssertionError(f"Unable to derive reverse tag for user={user!r} host={host!r}")
    return f"{user_label}{host_label}{REVERSE_SUFFIX}"


def _sanitize_host(host: str):
    sanitized = _sanitize_label(host)
    for char in sanitized:
        yield char


def _sanitize_label(value: str) -> str:
    cleaned = value.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-" and not last_dash:
            result.append("-")
            last_dash = True
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    return "".join(result).strip("-")


def assert_outbound(
    data: dict,
    host: str,
    password: str,
    email: str,
    server_name: str,
    *,
    address: str | None = None,
    allow_insecure: bool = False,
    pinned_peer_sha256: str | None = None,
    verify_peer_name: str | None = None,
) -> None:
    tag = expected_proxy_tag(host)
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") != tag:
            continue
        server = outbound["settings"]["servers"][0]
        expected_address = address or host
        assert server["address"] == expected_address
        assert server["password"] == password
        assert server["email"] == email
        tls_settings = outbound["streamSettings"]["tlsSettings"]
        assert tls_settings["serverName"] == server_name
        if pinned_peer_sha256 is not None:
            actual_pin = tls_settings.get("pinnedPeerCertSha256")
            if pinned_peer_sha256:
                assert actual_pin == pinned_peer_sha256
            else:
                assert actual_pin, "Expected pinnedPeerCertSha256 to be set"
            if verify_peer_name:
                assert tls_settings.get("verifyPeerCertByName") == verify_peer_name
            assert "allowInsecure" not in tls_settings or not tls_settings.get("allowInsecure")
        else:
            assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)
        return
    raise AssertionError(f"Outbound {tag} for host {host} not found")


def assert_routing_rule(data: dict, host: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == "direct" and host in rule.get("ip", []):
            return
    raise AssertionError(f"Routing rule for {host} -> direct not found")


def assert_redirect_rule(data: dict, cidr: str, tag: str) -> None:
    normalized = cidr.strip()
    if not normalized:
        raise AssertionError("CIDR value is empty")
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        ips = rule.get("ip") or []
        if isinstance(ips, list) and len(ips) == 1 and ips[0] == normalized:
            return
    raise AssertionError(f"Redirect rule for {normalized} via {tag} not found")


def assert_domain_redirect_rule(data: dict, domain: str, tag: str) -> None:
    normalized = domain.strip().lower()
    if not normalized:
        raise AssertionError("Domain value is empty")
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        domains = rule.get("domains") or []
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if normalized in lowered:
            return
    raise AssertionError(f"Domain redirect rule for {normalized} via {tag} not found")


def assert_no_redirect_rule(data: dict, cidr: str, tag: str | None = None) -> None:
    normalized = cidr.strip()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if tag and rule.get("outboundTag") != tag:
            continue
        ips = rule.get("ip") or []
        if isinstance(ips, list) and normalized in ips:
            raise AssertionError(f"Unexpected redirect rule for {normalized} via {rule.get('outboundTag')}")


def assert_no_domain_redirect_rule(data: dict, domain: str, tag: str | None = None) -> None:
    normalized = domain.strip().lower()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if tag and rule.get("outboundTag") != tag:
            continue
        domains = rule.get("domains") or []
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if normalized in lowered:
            raise AssertionError(f"Unexpected domain redirect rule for {domain} via {rule.get('outboundTag')}")


def assert_server_reverse_state(state: dict, reverse_tag: str, *, user: str | None = None, host: str | None = None) -> None:
    channels = state.get("reverse_channels")
    if not isinstance(channels, dict):
        raise AssertionError("Server config is missing reverse_channels")
    entry = channels.get(reverse_tag)
    if not isinstance(entry, dict):
        raise AssertionError(f"Reverse entry {reverse_tag} not recorded in server state")
    if user:
        recorded_user = (entry.get("user_id") or "").strip().lower()
        if recorded_user != user.strip().lower():
            raise AssertionError(f"Reverse entry {reverse_tag} recorded for unexpected user {recorded_user}")
    if host:
        recorded_host = (entry.get("host") or "").strip().lower()
        if recorded_host != host.strip().lower():
            raise AssertionError(f"Reverse entry {reverse_tag} recorded for unexpected host {recorded_host}")
    domain = entry.get("domain")
    if domain != reverse_tag:
        raise AssertionError(f"Reverse entry {reverse_tag} recorded domain {domain}")


def assert_server_reverse_routing(routing: dict, reverse_tag: str, *, user: str | None = None) -> None:
    reverse = routing.get("reverse", {})
    portals = reverse.get("portals") or []
    found_portal = False
    for raw in portals:
        if not isinstance(raw, dict):
            continue
        if raw.get("tag") == reverse_tag and raw.get("domain") == reverse_tag:
            found_portal = True
            break
    if not found_portal:
        raise AssertionError(f"Reverse portal {reverse_tag} not found in server routing config")

    rules = routing.get("routing", {}).get("rules", [])
    expected_domain = f"full:{reverse_tag}"
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        outbound = (rule.get("outboundTag") or "").strip()
        domains = [entry.strip().lower() for entry in rule.get("domain") or [] if isinstance(entry, str)]
        users = [entry.strip().lower() for entry in rule.get("user") or [] if isinstance(entry, str)]
        if outbound == reverse_tag and expected_domain in domains:
            if user:
                trimmed_user = user.strip().lower()
                if trimmed_user and (len(users) != 1 or users[0] != trimmed_user):
                    continue
            return
    raise AssertionError(f"Reverse routing rule for {reverse_tag} not found in server routing config")


def assert_client_reverse_artifacts(routing: dict, reverse_tag: str, endpoint_tag: str) -> None:
    reverse = routing.get("reverse", {})
    bridges = reverse.get("bridges") or []
    for raw in bridges:
        if not isinstance(raw, dict):
            continue
        if raw.get("tag") == reverse_tag and raw.get("domain") == reverse_tag:
            break
    else:
        raise AssertionError(f"Reverse bridge {reverse_tag} not recorded in client routing config")

    rules = routing.get("routing", {}).get("rules", [])
    target_domain = f"full:{reverse_tag}"
    domain_rule_found = False
    direct_rule_found = False
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        outbound = (rule.get("outboundTag") or "").strip()
        inbound = [entry.strip() for entry in rule.get("inboundTag") or [] if isinstance(entry, str)]
        domains = [entry.strip().lower() for entry in rule.get("domain") or [] if isinstance(entry, str)]
        if outbound == endpoint_tag and target_domain in domains:
            domain_rule_found = True
        if outbound == "direct" and reverse_tag in inbound:
            direct_rule_found = True
    if not domain_rule_found:
        raise AssertionError(f"Client routing is missing reverse domain rule for {reverse_tag}")
    if not direct_rule_found:
        raise AssertionError(f"Client routing is missing reverse direct rule for {reverse_tag}")


def assert_client_reverse_state(
    state: dict,
    reverse_tag: str,
    *,
    endpoint_tag: str,
    user: str,
    host: str,
) -> None:
    reverse = state.get("reverse")
    if not isinstance(reverse, dict):
        raise AssertionError("Client config is missing reverse map")
    entry = reverse.get(reverse_tag)
    if not isinstance(entry, dict):
        raise AssertionError(f"Reverse entry {reverse_tag} not recorded in client state")
    if (entry.get("endpoint_tag") or "").strip() != endpoint_tag:
        raise AssertionError(f"Reverse entry {reverse_tag} routes through unexpected outbound {entry.get('endpoint_tag')}")
    if (entry.get("tag") or "") != reverse_tag:
        raise AssertionError(f"Reverse entry {reverse_tag} recorded tag {entry.get('tag')}")
    if (entry.get("domain") or "") != reverse_tag:
        raise AssertionError(f"Reverse entry {reverse_tag} recorded domain {entry.get('domain')}")
    if (entry.get("user_id") or "").strip().lower() != user.strip().lower():
        raise AssertionError(f"Reverse entry {reverse_tag} recorded unexpected user {entry.get('user_id')}")
    if (entry.get("host") or "").strip().lower() != host.strip().lower():
        raise AssertionError(f"Reverse entry {reverse_tag} recorded unexpected host {entry.get('host')}")


def assert_server_redirect_rule(routing: dict, target: str, outbound_tag: str) -> None:
    normalized = target.strip().lower()
    rules = routing.get("routing", {}).get("rules", [])
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        if (rule.get("outboundTag") or "").strip() != outbound_tag:
            continue
        domain_entries = [entry.strip().lower() for entry in rule.get("domains") or [] if isinstance(entry, str)]
        ip_entries = [entry.strip().lower() for entry in rule.get("ip") or [] if isinstance(entry, str)]
        if normalized in domain_entries or normalized in ip_entries:
            return
    raise AssertionError(f"Server routing is missing redirect rule for {target} via {outbound_tag}")


def assert_server_redirect_state(state: dict, target: str, outbound_tag: str) -> None:
    redirects = state.get("server_redirects")
    if not isinstance(redirects, list):
        raise AssertionError("Server config is missing server_redirects list")
    normalized = target.strip().lower()
    for entry in redirects:
        if not isinstance(entry, dict):
            continue
        recorded_tag = (entry.get("outbound_tag") or entry.get("outboundTag") or "").strip()
        if recorded_tag != outbound_tag:
            continue
        domain_value = (entry.get("domain") or "").strip().lower()
        cidr_value = (entry.get("cidr") or "").strip().lower()
        if normalized in (domain_value, cidr_value):
            return
    raise AssertionError(f"Server state is missing redirect for {target} via {outbound_tag}")


def dump_failure_state(host: Host, label: str) -> None:
    print(f"==== FAILURE DUMP ({label}) on {host.backend.hostname} ====")
    script = " ; ".join(
        (
            "echo '--- xp2p tree ---'",
            "ls -la /etc/xp2p 2>/dev/null || true",
            "find /etc/xp2p -maxdepth 4 -print 2>/dev/null || true",
            "echo '--- xp2p apply dir ---'",
            "ls -la /etc/xp2p/.state 2>/dev/null || true",
            "for f in /etc/xp2p/.state/apply.request /etc/xp2p/.state/apply.error; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "echo '--- xp2p state tree ---'",
            "find /etc/xp2p/.state -maxdepth 5 -print 2>/dev/null || true",
            "echo '--- xp2p configs ---'",
            "for f in /etc/xp2p/xp2p-client.toml /etc/xp2p/xp2p-server.toml; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "for f in /etc/xp2p/.state/pending/xp2p-client.toml /etc/xp2p/.state/pending/xp2p-server.toml; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "for f in /etc/xp2p/.state/live/xp2p-client.toml /etc/xp2p/.state/live/xp2p-server.toml; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "for f in /etc/xp2p/.state/lkg/xp2p-client.toml /etc/xp2p/.state/lkg/xp2p-server.toml; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "echo '--- xp2p state ---'",
            "for f in /etc/xp2p/*.state.json /etc/xp2p/*-state-*.json; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && cat \"$f\"; done",
            "echo '--- xp2p config dirs ---'",
            "for d in /etc/xp2p/config-client /etc/xp2p/config-server; do "
            "[ -d \"$d\" ] && ls -la \"$d\"; done",
            "for d in /etc/xp2p/config-client /etc/xp2p/config-server; do "
            "if [ -d \"$d\" ]; then "
            "for f in \"$d\"/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done; "
            "fi; done",
            "echo '--- xp2p pending config dirs ---'",
            "for d in /etc/xp2p/.state/pending/config-client /etc/xp2p/.state/pending/config-server; do "
            "[ -d \"$d\" ] && ls -la \"$d\"; done",
            "for d in /etc/xp2p/.state/pending/config-client /etc/xp2p/.state/pending/config-server; do "
            "if [ -d \"$d\" ]; then "
            "for f in \"$d\"/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done; "
            "fi; done",
            "echo '--- xp2p live config dirs ---'",
            "for d in /etc/xp2p/.state/live/config-client /etc/xp2p/.state/live/config-server; do "
            "[ -d \"$d\" ] && ls -la \"$d\"; done",
            "for d in /etc/xp2p/.state/live/config-client /etc/xp2p/.state/live/config-server; do "
            "if [ -d \"$d\" ]; then "
            "for f in \"$d\"/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done; "
            "fi; done",
            "echo '--- xp2p lkg config dirs ---'",
            "for d in /etc/xp2p/.state/lkg/config-client /etc/xp2p/.state/lkg/config-server; do "
            "[ -d \"$d\" ] && ls -la \"$d\"; done",
            "for d in /etc/xp2p/.state/lkg/config-client /etc/xp2p/.state/lkg/config-server; do "
            "if [ -d \"$d\" ]; then "
            "for f in \"$d\"/*.json; do [ -f \"$f\" ] || continue; echo \"--- $f ---\"; cat \"$f\"; done; "
            "fi; done",
            "echo '--- xp2p logs ---'",
            "find /var/log/xp2p -maxdepth 3 -print 2>/dev/null || true",
            "for f in /var/log/xp2p/client/service.log /var/log/xp2p/server/service.log; do "
            "[ -f \"$f\" ] && echo \"--- $f ---\" && tail -n 200 \"$f\"; done",
            "echo '--- processes (xp2p/xray) ---'",
            "ps auxww 2>/dev/null | head -n 5 || true",
            "ps auxww 2>/dev/null | grep -E '(^|/)(xp2p|xray)(\\s|$)' | head -n 200 || true",
            "pgrep -af xp2p 2>/dev/null || true",
            "pgrep -af xray 2>/dev/null || true",
            "echo '--- sockets ---'",
            "if command -v ss >/dev/null 2>&1; then ss -ltnp 2>/dev/null || true; fi",
            "echo '--- systemd status ---'",
            "systemctl --no-pager --full status xp2p-client 2>/dev/null || true",
            "systemctl --no-pager --full status xp2p-server 2>/dev/null || true",
            "echo '--- journalctl (xp2p-client) ---'",
            "journalctl --no-pager -u xp2p-client -n 200 2>/dev/null || true",
            "echo '--- journalctl (xp2p-server) ---'",
            "journalctl --no-pager -u xp2p-server -n 200 2>/dev/null || true",
            "true",
        )
    )
    result = host.run(f"sudo -n /bin/sh -c {shlex.quote(script)}")
    encoding = getattr(sys.stdout, "encoding", None) or "utf-8"
    if result.stdout:
        print(result.stdout.encode(encoding, errors="replace").decode(encoding, errors="replace"))
    if result.stderr:
        print(result.stderr.encode(encoding, errors="replace").decode(encoding, errors="replace"))
    print("==== END FAILURE DUMP ====")


def expected_forward_tag(port: int) -> str:
    if port <= 0:
        raise AssertionError("Listen port must be positive")
    return f"in_{int(port)}"


def expected_forward_remark(host: str, port: int) -> str:
    trimmed = host.strip()
    if not trimmed:
        raise AssertionError("Target host is empty")
    return f"forward:{trimmed}:{int(port)}"


def forward_network_value(protocol: str) -> str:
    normalized = (protocol or "").strip().lower()
    if normalized == "tcp":
        return "tcp"
    if normalized == "udp":
        return "udp"
    return "tcp,udp"


def assert_forward_rule_entry(
    entries: list[dict],
    listen_port: int,
    *,
    listen_address: str,
    target_host: str,
    target_port: int,
    protocol: str,
) -> dict:
    listen = listen_port
    addr = listen_address.strip()
    host = target_host.strip()
    proto = (protocol or "").strip().lower() or "both"
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_port = int(entry.get("listen_port") or entry.get("listenPort") or 0)
        if recorded_port != listen:
            continue
        recorded_addr = (entry.get("listen_address") or entry.get("listenAddress") or "").strip()
        recorded_host = (entry.get("target_host") or "").strip()
        recorded_port_target = int(entry.get("target_port") or entry.get("targetPort") or 0)
        recorded_proto = (entry.get("protocol") or "").strip().lower()
        if recorded_addr != addr:
            continue
        if recorded_host != host:
            continue
        if recorded_port_target != target_port:
            continue
        if recorded_proto != proto:
            continue
        return entry
    raise AssertionError(f"Forward entry on {addr}:{listen} targeting {host}:{target_port} not found")


def assert_no_forward_rule_entry(entries: list[dict], listen_port: int) -> None:
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_port = int(entry.get("listen_port") or entry.get("listenPort") or 0)
        if recorded_port == listen_port:
            raise AssertionError(f"Unexpected forward entry recorded on port {listen_port}")


def assert_forward_inbound_entry(
    data: dict,
    listen_port: int,
    *,
    listen_address: str,
    target_host: str,
    target_port: int,
    protocol: str,
) -> None:
    listen = listen_address.strip()
    host = target_host.strip()
    network = forward_network_value(protocol)
    for entry in data.get("inbounds", []) or []:
        if not isinstance(entry, dict):
            continue
        if entry.get("protocol") != "dokodemo-door":
            continue
        if int(entry.get("port") or 0) != listen_port:
            continue
        recorded_listen = (entry.get("listen") or "").strip()
        if recorded_listen != listen:
            continue
        settings = entry.get("settings") or {}
        recorded_host = (settings.get("address") or "").strip()
        recorded_port = int(settings.get("port") or 0)
        recorded_network = (settings.get("network") or "").strip().lower()
        if recorded_host == host and recorded_port == target_port and recorded_network == network:
            return
    raise AssertionError(f"dokodemo-door inbound on {listen}:{listen_port} not found")


def assert_no_forward_inbound_entry(data: dict, listen_port: int) -> None:
    for entry in data.get("inbounds", []) or []:
        if not isinstance(entry, dict):
            continue
        if entry.get("protocol") != "dokodemo-door":
            continue
        if int(entry.get("port") or 0) == listen_port:
            raise AssertionError(f"Unexpected dokodemo-door inbound present on port {listen_port}")
