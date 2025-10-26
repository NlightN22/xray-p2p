import re
import shlex
from dataclasses import dataclass
from typing import Dict, List
from urllib.parse import urlsplit

import pytest

from .helpers import (
    client_install,
    client_remove,
    load_inbounds_config,
    run_checked,
    server_remove,
)

CLIENT_USER_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client_user.sh"
REDIRECT_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/redirect.sh"

PRIMARY_TROJAN_URL = (
    "trojan://pytest-primary-pass@primary.example.test:11443"
    "?security=tls&allowInsecure=1#pytest-primary"
)
SECONDARY_TROJAN_URL = (
    "trojan://pytest-secondary-pass@backup-secondary.example.test:22443"
    "?security=tls&allowInsecure=1&network=tcp#pytest-secondary"
)
SECONDARY_SUBNET = "10.201.51.0/24"

SCRIPT_BASE_ENV = {
    "XRAY_SKIP_REPO_CHECK": "1",
}


@dataclass
class ClientEntry:
    tag: str
    protocol: str
    target: str
    network: str
    security: str


@dataclass
class RedirectEntry:
    subnet: str
    port: str


def _run_repo_script(host, url: str, args: List[str], env: Dict[str, str] | None, description: str):
    tokens = " ".join(shlex.quote(arg) for arg in args)
    script_command = f"curl -fsSL {shlex.quote(url)} | sh -s --"
    if tokens:
        script_command = f"{script_command} {tokens}"

    pieces: List[str] = []
    effective_env = {**SCRIPT_BASE_ENV}
    if env:
        effective_env.update(env)
    for key, value in sorted(effective_env.items()):
        pieces.append(f"export {key}={shlex.quote(str(value))}")
    pieces.append(script_command)

    command = "sh -c " + shlex.quote("; ".join(pieces))
    return run_checked(host, command, description)


def client_user_exec(host, *args: str, env: Dict[str, str] | None = None):
    description = "client_user " + (" ".join(args) if args else "list")
    return _run_repo_script(host, CLIENT_USER_SCRIPT_URL, list(args), env, description)


def redirect_exec(host, *args: str, env: Dict[str, str] | None = None):
    description = "redirect " + (" ".join(args) if args else "list")
    return _run_repo_script(host, REDIRECT_SCRIPT_URL, list(args), env, description)


def _parse_client_entries(output: str) -> List[ClientEntry]:
    lines = [line.rstrip() for line in output.splitlines() if line.strip()]
    if not lines:
        return []
    if lines[0].startswith("No client outbounds configured."):
        return []
    header = lines[0]
    assert header.startswith("Tag\tProtocol\tTarget"), f"Unexpected client_user header: {header}"
    entries: List[ClientEntry] = []
    for line in lines[1:]:
        parts = line.split("\t")
        if len(parts) != 5:
            continue
        entries.append(ClientEntry(*parts))
    return entries


def _parse_redirect_entries(output: str) -> List[RedirectEntry]:
    lines = [line.rstrip() for line in output.splitlines() if line.strip()]
    if not lines:
        return []
    if lines[0].startswith("No transparent redirect entries found."):
        return []
    entries: List[RedirectEntry] = []
    for line in lines[2:]:
        if re.fullmatch(r"-+", line):
            continue
        parts = line.split()
        if len(parts) < 2:
            continue
        subnet = parts[0]
        port = parts[-1]
        entries.append(RedirectEntry(subnet=subnet, port=port))
    return entries


def client_user_list(host) -> List[ClientEntry]:
    result = client_user_exec(host, "list")
    return _parse_client_entries(result.stdout)


def redirect_list(host) -> List[RedirectEntry]:
    result = redirect_exec(host, "list")
    return _parse_redirect_entries(result.stdout)


def _sanitize_tag(hostname: str) -> str:
    lowered = hostname.lower()
    sanitized = re.sub(r"[^0-9a-z._-]+", "-", lowered).strip("-")
    return sanitized or "proxy"


def connection_tag(url: str) -> str:
    parts = urlsplit(url)
    hostname = parts.hostname or ""
    port = parts.port
    if port is None:
        raise ValueError(f"Connection string lacks port: {url}")
    return f"{_sanitize_tag(hostname)}-{port}"


def connection_target(url: str) -> str:
    parts = urlsplit(url)
    hostname = parts.hostname or ""
    port = parts.port
    if port is None:
        raise ValueError(f"Connection string lacks port: {url}")
    return f"{hostname}:{port}"


def _extract_dokodemo_port(host) -> int:
    inbounds = load_inbounds_config(host)
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        if inbound.get("protocol") != "dokodemo-door":
            continue
        raw_port = inbound.get("port")
        if isinstance(raw_port, int):
            return raw_port
        if isinstance(raw_port, str) and raw_port.isdigit():
            return int(raw_port)
    raise AssertionError("No dokodemo-door inbound port discovered for redirect configuration")


def _collect_trojan(entries: List[ClientEntry]) -> List[ClientEntry]:
    return [entry for entry in entries if entry.protocol == "trojan"]


@pytest.mark.serial
def test_client_user_manages_multiple_tunnels(host_r2):
    server_remove(host_r2, purge_core=True, check=False)
    client_remove(host_r2, purge_core=True, check=False)

    install_env = {
        "XRAY_REISSUE_CERT": "1",
        "XRAY_SKIP_PORT_CHECK": "1",
    }

    primary_tag_expected = connection_tag(PRIMARY_TROJAN_URL)
    primary_target_expected = connection_target(PRIMARY_TROJAN_URL)
    secondary_tag_expected = connection_tag(SECONDARY_TROJAN_URL)
    secondary_target_expected = connection_target(SECONDARY_TROJAN_URL)

    try:
        client_install(host_r2, PRIMARY_TROJAN_URL, "--force", env=install_env)

        redirect_port = _extract_dokodemo_port(host_r2)

        client_entries_initial = client_user_list(host_r2)
        trojan_initial = _collect_trojan(client_entries_initial)
        assert len(trojan_initial) == 1, "Initial install should configure exactly one trojan outbound"
        primary_entry = trojan_initial[0]
        assert primary_entry.tag == primary_tag_expected, "Primary outbound tag mismatch"
        assert primary_entry.target == primary_target_expected, "Primary outbound target mismatch"

        redirect_entries_initial = redirect_list(host_r2)

        client_user_exec(
            host_r2,
            "add",
            SECONDARY_TROJAN_URL,
            SECONDARY_SUBNET,
            str(redirect_port),
        )

        client_entries_after_second = client_user_list(host_r2)
        trojan_after_second = _collect_trojan(client_entries_after_second)
        assert len(trojan_after_second) == 2, "Second add should result in two trojan outbounds"
        secondary_entry = next(
            entry for entry in trojan_after_second if entry.tag != primary_entry.tag
        )
        assert secondary_entry.tag == secondary_tag_expected, "Secondary outbound tag mismatch after add"
        assert secondary_entry.target == secondary_target_expected, "Secondary outbound target mismatch after add"

        redirect_entries_after_second = redirect_list(host_r2)
        assert len(redirect_entries_after_second) == len(redirect_entries_initial) + 1, "Redirect entries should grow after adding subnet"
        assert any(entry.subnet == SECONDARY_SUBNET for entry in redirect_entries_after_second), "Secondary subnet missing from redirect list"

        client_user_exec(host_r2, "remove", secondary_entry.tag)

        client_entries_after_remove_second = client_user_list(host_r2)
        trojan_after_remove_second = _collect_trojan(client_entries_after_remove_second)
        assert len(trojan_after_remove_second) == len(trojan_initial), "Removing secondary should restore initial trojan count"
        assert {entry.tag for entry in trojan_after_remove_second} == {primary_entry.tag}, "Primary outbound should remain after removing secondary"

        redirect_entries_after_remove_second = redirect_list(host_r2)
        assert len(redirect_entries_after_remove_second) == len(redirect_entries_initial), "Redirect count should revert after removing secondary"

        client_user_exec(
            host_r2,
            "add",
            SECONDARY_TROJAN_URL,
            SECONDARY_SUBNET,
            str(redirect_port),
        )

        client_entries_after_readd = client_user_list(host_r2)
        trojan_after_readd = _collect_trojan(client_entries_after_readd)
        assert len(trojan_after_readd) == len(trojan_initial) + 1, "Re-adding secondary should restore two trojan outbounds"
        assert any(entry.tag == secondary_tag_expected for entry in trojan_after_readd), "Secondary outbound missing after re-add"

        redirect_entries_after_readd = redirect_list(host_r2)
        assert len(redirect_entries_after_readd) == len(redirect_entries_initial) + 1, "Redirect count should grow again after re-adding secondary"

        client_user_exec(host_r2, "remove", primary_entry.tag)

        client_entries_final = client_user_list(host_r2)
        trojan_final = _collect_trojan(client_entries_final)
        assert len(trojan_final) == len(trojan_initial), "Removing primary should leave one trojan outbound (the secondary)"
        assert {entry.tag for entry in trojan_final} == {secondary_tag_expected}, "Secondary outbound should remain after removing primary"
        assert len(client_entries_final) == len(client_entries_initial), "Total outbound count should stay consistent with initial state"

        redirect_entries_final = redirect_list(host_r2)
        assert len(redirect_entries_final) == len(redirect_entries_initial) + 1, "Secondary redirect should remain after removing primary outbound"
        assert any(entry.subnet == SECONDARY_SUBNET for entry in redirect_entries_final), "Secondary subnet should persist in redirect list"
    finally:
        client_remove(host_r2, purge_core=True, check=False)
        server_remove(host_r2, purge_core=True, check=False)
