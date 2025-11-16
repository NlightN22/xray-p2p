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
REVERSE_SUFFIX = ".rev"


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


def read_first_existing_json(host: Host, paths: list[PurePosixPath]) -> dict:
    for path in paths:
        if linux_env.path_exists(host, path):
            return read_json(host, path)
    raise AssertionError(f"None of the state files exist: {paths}")


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
        raise AssertionError("Server install-state is missing reverse_channels")
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
        inbound = [entry.strip() for entry in rule.get("inboundTag") or [] if isinstance(entry, str)]
        domains = [entry.strip().lower() for entry in rule.get("domain") or [] if isinstance(entry, str)]
        users = [entry.strip().lower() for entry in rule.get("user") or [] if isinstance(entry, str)]
        if (
            outbound == reverse_tag
            and len(inbound) == 1
            and inbound[0] == reverse_tag
            and len(domains) == 1
            and domains[0] == expected_domain
        ):
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
        raise AssertionError("Client install-state is missing reverse map")
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
        raise AssertionError("Server install-state is missing server_redirects list")
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


def expected_forward_tag(port: int) -> str:
    if port <= 0:
        raise AssertionError("Listen port must be positive")
    return f"in_{int(port)}"


def expected_forward_remark(ip: str, port: int) -> str:
    trimmed = ip.strip()
    if not trimmed:
        raise AssertionError("Target IP is empty")
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
    target_ip: str,
    target_port: int,
    protocol: str,
) -> dict:
    listen = listen_port
    addr = listen_address.strip()
    ip = target_ip.strip()
    proto = (protocol or "").strip().lower() or "both"
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_port = int(entry.get("listen_port") or entry.get("listenPort") or 0)
        if recorded_port != listen:
            continue
        recorded_addr = (entry.get("listen_address") or entry.get("listenAddress") or "").strip()
        recorded_ip = (entry.get("target_ip") or entry.get("targetIP") or "").strip()
        recorded_port_target = int(entry.get("target_port") or entry.get("targetPort") or 0)
        recorded_proto = (entry.get("protocol") or "").strip().lower()
        if recorded_addr != addr:
            continue
        if recorded_ip != ip:
            continue
        if recorded_port_target != target_port:
            continue
        if recorded_proto != proto:
            continue
        return entry
    raise AssertionError(f"Forward entry on {addr}:{listen} targeting {ip}:{target_port} not found")


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
    target_ip: str,
    target_port: int,
    protocol: str,
) -> None:
    listen = listen_address.strip()
    ip = target_ip.strip()
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
        recorded_ip = (settings.get("address") or "").strip()
        recorded_port = int(settings.get("port") or 0)
        recorded_network = (settings.get("network") or "").strip().lower()
        if recorded_ip == ip and recorded_port == target_port and recorded_network == network:
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
