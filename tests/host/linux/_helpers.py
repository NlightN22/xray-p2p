from __future__ import annotations

from pathlib import PurePosixPath

from testinfra.host import Host

from tests.host.linux import env as linux_env

INSTALL_ROOT = PurePosixPath("/etc/xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
SERVER_CONFIG_DIR_NAME = "config-server"
CLIENT_CONFIG_DIR = INSTALL_ROOT / CLIENT_CONFIG_DIR_NAME
SERVER_CONFIG_DIR = INSTALL_ROOT / SERVER_CONFIG_DIR_NAME
CLIENT_STATE_FILES = [
    INSTALL_ROOT / "install-state-client.json",
    INSTALL_ROOT / "install-state.json",
]
SERVER_STATE_FILES = [
    INSTALL_ROOT / "install-state-server.json",
    INSTALL_ROOT / "install-state.json",
]
LOG_ROOT = PurePosixPath("/var/log/xp2p")
CLIENT_LOG_FILE = LOG_ROOT / "client.err"
SERVER_LOG_FILE = LOG_ROOT / "server.err"
XRAY_BINARY = INSTALL_ROOT / "bin" / "xray"


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
    )


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
    )


def read_json(host: Host, path: PurePosixPath) -> dict:
    return linux_env.read_json(host, path)


def read_text(host: Host, path: PurePosixPath) -> str:
    return linux_env.read_text(host, path)


def path_exists(host: Host, path: PurePosixPath) -> bool:
    return linux_env.path_exists(host, path)


def remove_path(host: Host, path: PurePosixPath) -> None:
    linux_env.remove_path(host, path)


def write_text(host: Host, path: PurePosixPath, content: str) -> None:
    linux_env.write_text(host, path, content)


def file_sha256(host: Host, path: PurePosixPath) -> str:
    return linux_env.file_sha256(host, path)


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


def _sanitize_host(host: str):
    host = host.strip().lower()
    last_dash = False
    for char in host:
        if char.isalnum():
            yield char
            last_dash = False
            continue
        if char == "-" and not last_dash:
            yield "-"
            last_dash = True
            continue
        if not last_dash:
            yield "-"
            last_dash = True


def assert_outbound(
    data: dict,
    host: str,
    password: str,
    email: str,
    server_name: str,
    *,
    allow_insecure: bool = False,
) -> None:
    tag = expected_proxy_tag(host)
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") != tag:
            continue
        server = outbound["settings"]["servers"][0]
        assert server["address"] == host
        assert server["password"] == password
        assert server["email"] == email
        tls_settings = outbound["streamSettings"]["tlsSettings"]
        assert tls_settings["serverName"] == server_name
        assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)
        return
    raise AssertionError(f"Outbound {tag} for host {host} not found")


def assert_routing_rule(data: dict, host: str) -> None:
    tag = expected_proxy_tag(host)
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == tag and host in rule.get("ip", []):
            return
    raise AssertionError(f"Routing rule for {host} -> {tag} not found")


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


def assert_no_redirect_rule(data: dict, cidr: str, tag: str | None = None) -> None:
    normalized = cidr.strip()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if tag and rule.get("outboundTag") != tag:
            continue
        ips = rule.get("ip") or []
        if isinstance(ips, list) and normalized in ips:
            raise AssertionError(f"Unexpected redirect rule for {normalized} via {rule.get('outboundTag')}")
