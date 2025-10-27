import re
import shlex
from dataclasses import dataclass
from typing import Dict, Iterator, List, Optional, Set
from urllib.parse import urlsplit

import pytest

from .helpers import (
    client_install,
    client_remove,
    load_inbounds_config,
    run_checked,
    server_remove,
)

pytestmark = pytest.mark.serial

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
    "XRAY_FORCE_CONFIG": "1",
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


@dataclass
class ClientUserState:
    host: object
    redirect_port: int
    initial_redirects: Set[str]
    primary_tag: str
    primary_target: str
    secondary_target: str
    secondary_tag: Optional[str] = None


def _run_repo_script(
    host,
    url: str,
    args: List[str],
    env: Dict[str, str] | None,
    description: str,
    *,
    check: bool = True,
):
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
    if check:
        return run_checked(host, command, description)
    return host.run(command)


def client_user_exec(
    host,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
):
    description = "client_user " + (" ".join(args) if args else "list")
    return _run_repo_script(
        host,
        CLIENT_USER_SCRIPT_URL,
        list(args),
        env,
        description,
        check=check,
    )


def redirect_exec(
    host,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
):
    description = "redirect " + (" ".join(args) if args else "list")
    return _run_repo_script(
        host,
        REDIRECT_SCRIPT_URL,
        list(args),
        env,
        description,
        check=check,
    )


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


def _find_trojan_by_target(entries: List[ClientEntry], target: str) -> Optional[ClientEntry]:
    for entry in entries:
        if entry.target == target and entry.protocol == "trojan":
            return entry
    return None


def _redirect_subnet_set(host) -> Set[str]:
    return {entry.subnet for entry in redirect_list(host)}


@pytest.fixture(scope="module")
def client_user_state(host_r2) -> Iterator[ClientUserState]:
    server_remove(host_r2, purge_core=True, check=False)
    client_remove(host_r2, purge_core=True, check=False)

    install_env = {
        "XRAY_REISSUE_CERT": "1",
        "XRAY_SKIP_PORT_CHECK": "1",
    }

    client_install(host_r2, PRIMARY_TROJAN_URL, "--force", env=install_env)

    redirect_port = _extract_dokodemo_port(host_r2)
    entries = client_user_list(host_r2)
    trojan_entries = _collect_trojan(entries)
    assert len(trojan_entries) == 1, "Primary install should seed exactly one trojan outbound"
    primary_entry = trojan_entries[0]

    state = ClientUserState(
        host=host_r2,
        redirect_port=redirect_port,
        initial_redirects=_redirect_subnet_set(host_r2),
        primary_tag=primary_entry.tag,
        primary_target=primary_entry.target,
        secondary_target=connection_target(SECONDARY_TROJAN_URL),
    )

    try:
        yield state
    finally:
        if state.secondary_tag:
            client_user_exec(host_r2, "remove", state.secondary_tag, check=False)
        client_user_exec(host_r2, "remove", state.primary_tag, check=False)
        client_remove(host_r2, purge_core=True, check=False)
        server_remove(host_r2, purge_core=True, check=False)


def _ensure_secondary_absent(state: ClientUserState):
    entries = client_user_list(state.host)
    existing = _find_trojan_by_target(entries, state.secondary_target)
    if existing:
        client_user_exec(state.host, "remove", existing.tag, check=False)
    state.secondary_tag = None


def _ensure_secondary_present(state: ClientUserState) -> ClientEntry:
    entries = client_user_list(state.host)
    existing = _find_trojan_by_target(entries, state.secondary_target)
    if not existing:
        client_user_exec(
            state.host,
            "add",
            SECONDARY_TROJAN_URL,
            SECONDARY_SUBNET,
            str(state.redirect_port),
        )
        entries = client_user_list(state.host)
        existing = _find_trojan_by_target(entries, state.secondary_target)
        assert existing is not None, "Secondary outbound should be present after add"
    state.secondary_tag = existing.tag
    return existing


@pytest.mark.incremental
class TestClientUserFlow:
    def test_stage_01_primary_present(self, client_user_state: ClientUserState):
        entries = client_user_list(client_user_state.host)
        trojan_entries = _collect_trojan(entries)
        assert len(trojan_entries) == 1, "Only primary outbound expected initially"
        primary_entry = trojan_entries[0]
        assert primary_entry.target == client_user_state.primary_target, "Primary outbound target mismatch"
        client_user_state.primary_tag = primary_entry.tag
        assert (
            _redirect_subnet_set(client_user_state.host) == client_user_state.initial_redirects
        ), "Initial redirect set mismatch"

    def test_stage_02_add_secondary(self, client_user_state: ClientUserState):
        _ensure_secondary_absent(client_user_state)
        client_user_exec(
            client_user_state.host,
            "add",
            SECONDARY_TROJAN_URL,
            SECONDARY_SUBNET,
            str(client_user_state.redirect_port),
        )
        entries = client_user_list(client_user_state.host)
        trojan_entries = _collect_trojan(entries)
        assert len(trojan_entries) == 2, "Secondary outbound not added"
        secondary_entry = _find_trojan_by_target(trojan_entries, client_user_state.secondary_target)
        assert secondary_entry is not None, "Secondary outbound target missing after add"
        client_user_state.secondary_tag = secondary_entry.tag

        redirect_set = _redirect_subnet_set(client_user_state.host)
        expected = set(client_user_state.initial_redirects)
        expected.add(SECONDARY_SUBNET)
        assert redirect_set == expected, "Redirect set should include secondary subnet after add"

    def test_stage_03_remove_secondary(self, client_user_state: ClientUserState):
        secondary_entry = _ensure_secondary_present(client_user_state)
        remove_result = client_user_exec(
            client_user_state.host,
            "remove",
            secondary_entry.tag,
            check=False,
        )
        assert remove_result.rc in (0, 1), (
            f"Secondary removal unexpected rc={remove_result.rc}\n"
            f"stdout:\n{remove_result.stdout}\n"
            f"stderr:\n{remove_result.stderr}"
        )
        client_user_state.secondary_tag = None

        entries = client_user_list(client_user_state.host)
        trojan_entries = _collect_trojan(entries)
        assert len(trojan_entries) == 1, "Primary outbound should be the only entry after removing secondary"
        assert trojan_entries[0].target == client_user_state.primary_target, "Primary outbound should remain"

        redirect_set = _redirect_subnet_set(client_user_state.host)
        assert redirect_set == client_user_state.initial_redirects, "Redirect set should revert after removing secondary"

    def test_stage_04_readd_secondary(self, client_user_state: ClientUserState):
        _ensure_secondary_absent(client_user_state)
        secondary_entry = _ensure_secondary_present(client_user_state)
        assert (
            secondary_entry.target == client_user_state.secondary_target
        ), "Secondary outbound target mismatch after re-add"

        redirect_set = _redirect_subnet_set(client_user_state.host)
        expected = set(client_user_state.initial_redirects)
        expected.add(SECONDARY_SUBNET)
        assert redirect_set == expected, "Redirect set should reflect secondary subnet after re-add"

    def test_stage_05_remove_primary_leaves_secondary(self, client_user_state: ClientUserState):
        secondary_entry = _ensure_secondary_present(client_user_state)
        remove_primary = client_user_exec(
            client_user_state.host,
            "remove",
            client_user_state.primary_tag,
            check=False,
        )
        assert remove_primary.rc in (0, 1), (
            f"Primary removal unexpected rc={remove_primary.rc}\n"
            f"stdout:\n{remove_primary.stdout}\n"
            f"stderr:\n{remove_primary.stderr}"
        )

        entries = client_user_list(client_user_state.host)
        trojan_entries = _collect_trojan(entries)
        assert len(trojan_entries) == 1, "Secondary outbound should remain after removing primary"
        remaining = trojan_entries[0]
        assert remaining.target == client_user_state.secondary_target, "Remaining outbound should be the secondary entry"

        redirect_set = _redirect_subnet_set(client_user_state.host)
        expected = set(client_user_state.initial_redirects)
        expected.add(SECONDARY_SUBNET)
        assert redirect_set == expected, "Secondary redirect should persist after removing primary"
