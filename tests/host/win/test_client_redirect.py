import json
from pathlib import Path

import pytest

from tests.host.win import env as _env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = CLIENT_INSTALL_DIR / CLIENT_CONFIG_DIR_NAME
CLIENT_ROUTING_JSON = CLIENT_CONFIG_DIR / "routing.json"
CLIENT_STATE_FILE = CLIENT_INSTALL_DIR / "install-state-client.json"
PRIMARY_HOST = "10.120.0.10"
SECONDARY_HOST = "10.120.0.11"
REDIRECT_CIDR = "10.123.0.0/16"
REDIRECT_DOMAIN = "svc.internal.example"
INVALID_CIDR = "10.999.0.0/33"


def _cleanup_client_install(client_host, runner, msi_path: str) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.install_xp2p_from_msi(client_host, msi_path)


def _install_endpoint(runner, host: str, user: str, password: str) -> None:
    runner(
        "client",
        "install",
        "--server-address",
        host,
        "--user",
        user,
        "--password",
        password,
        check=True,
    )


def _read_remote_json(client_host, path: Path) -> dict:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Raw {quoted}
"""
    result = _env.run_powershell(client_host, script)
    assert result.rc == 0, (
        f"Failed to read remote JSON {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{result.stdout}")


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _assert_routing_rule(data: dict, host: str) -> None:
    tag = _expected_tag(host)
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        ips = rule.get("ip", [])
        if rule.get("outboundTag") == tag and host in ips:
            return
    raise AssertionError(f"Routing rule for {host} -> {tag} not found")


def _assert_redirect_rule(data: dict, cidr: str, tag: str) -> None:
    normalized = cidr.strip()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        ips = rule.get("ip", [])
        if isinstance(ips, list) and len(ips) == 1 and ips[0] == normalized:
            return
    raise AssertionError(f"Redirect rule for {normalized} via {tag} not found")


def _assert_no_redirect_rule(data: dict, cidr: str) -> None:
    normalized = cidr.strip()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        ips = rule.get("ip", [])
        if isinstance(ips, list) and normalized in ips:
            raise AssertionError(f"Unexpected redirect rule for {normalized}")


def _assert_domain_redirect_rule(data: dict, domain: str, tag: str) -> None:
    normalized = domain.strip().lower()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        domains = rule.get("domains", [])
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if normalized in lowered:
            return
    raise AssertionError(f"Domain redirect rule for {normalized} via {tag} not found")


def _assert_no_domain_redirect_rule(data: dict, domain: str) -> None:
    normalized = domain.strip().lower()
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        domains = rule.get("domains", [])
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if normalized in lowered:
            raise AssertionError(f"Unexpected domain redirect rule for {domain}")


def _redirect_cmd(runner, subcommand: str, *args: str, check: bool = False):
    base = ["client", "redirect", subcommand]
    base.extend(args)
    return runner(*base, check=check)


def _list_redirects(runner):
    result = _redirect_cmd(runner, "list", check=True)
    return result.stdout or "", _parse_redirect_output(result.stdout or "")


def _parse_redirect_output(text: str) -> list[dict[str, str]]:
    lines = [line.strip() for line in (text or "").splitlines() if line.strip()]
    header_idx = None
    legacy = False
    for idx, line in enumerate(lines):
        lowered = line.lower()
        if lowered.startswith("no redirect rules"):
            return []
        if lowered.startswith("type"):
            header_idx = idx
            break
        if lowered.startswith("cidr"):
            legacy = True
            header_idx = idx
            break
    if header_idx is None:
        raise AssertionError(f"Unexpected redirect output: {text!r}")

    entries: list[dict[str, str]] = []
    for row in lines[header_idx + 1 :]:
        parts = row.split()
        if legacy:
            if len(parts) < 3:
                continue
            entries.append({"type": "CIDR", "value": parts[0], "cidr": parts[0], "tag": parts[1], "host": parts[2]})
            continue
        if len(parts) < 4:
            continue
        entry = {"type": parts[0], "value": parts[1], "tag": parts[2], "host": parts[3]}
        if entry["type"].lower() == "cidr":
            entry["cidr"] = entry["value"]
        entries.append(entry)
    return entries


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()


@pytest.mark.host
@pytest.mark.win
def test_client_redirect_operations(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        _install_endpoint(xp2p_client_runner, PRIMARY_HOST, "primary@example.com", "win-primary-pass")
        _install_endpoint(xp2p_client_runner, SECONDARY_HOST, "secondary@example.com", "win-secondary-pass")

        output, records = _list_redirects(xp2p_client_runner)
        assert "no redirect rules configured" in output.lower()
        assert records == []

        primary_tag = _expected_tag(PRIMARY_HOST)
        secondary_tag = _expected_tag(SECONDARY_HOST)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--tag",
            primary_tag,
            check=True,
        )

        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(
            rec.get("cidr") == REDIRECT_CIDR and rec["tag"] == primary_tag and rec["type"].lower() == "cidr"
            for rec in records
        )

        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_redirect_rule(routing, REDIRECT_CIDR, primary_tag)
        _assert_routing_rule(routing, PRIMARY_HOST)
        _assert_routing_rule(routing, SECONDARY_HOST)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--cidr",
            REDIRECT_CIDR,
            check=True,
        )
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_no_redirect_rule(routing, REDIRECT_CIDR)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(rec["host"] == SECONDARY_HOST for rec in records)
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_no_redirect_rule(routing, REDIRECT_CIDR)

        invalid_tag_result = _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            INVALID_CIDR,
            "--tag",
            primary_tag,
            check=False,
        )
        assert invalid_tag_result.rc != 0
        assert "invalid cidr" in _combined_output(invalid_tag_result)

        invalid_host_result = _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            INVALID_CIDR,
            "--host",
            SECONDARY_HOST,
            check=False,
        )
        assert invalid_host_result.rc != 0
        assert "invalid cidr" in _combined_output(invalid_host_result)

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--domain",
            REDIRECT_DOMAIN,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_domain_redirect_rule(routing, REDIRECT_DOMAIN, secondary_tag)
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(
            rec["type"].lower() == "domain"
            and rec["value"].lower() == REDIRECT_DOMAIN
            and rec["host"] == SECONDARY_HOST
            for rec in records
        )

        _redirect_cmd(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)
        _assert_domain_redirect_rule(routing, REDIRECT_DOMAIN, secondary_tag)
        list_output, records = _list_redirects(xp2p_client_runner)
        assert any(rec.get("cidr") == REDIRECT_CIDR for rec in records)
        assert any(rec["value"].lower() == REDIRECT_DOMAIN for rec in records)

        _redirect_cmd(
            xp2p_client_runner,
            "remove",
            "--domain",
            REDIRECT_DOMAIN,
            "--host",
            SECONDARY_HOST,
            check=True,
        )
        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_no_domain_redirect_rule(routing, REDIRECT_DOMAIN)
        _assert_redirect_rule(routing, REDIRECT_CIDR, secondary_tag)

        xp2p_client_runner(
            "client",
            "remove",
            SECONDARY_HOST,
            check=True,
        )

        auto_output, auto_records = _list_redirects(xp2p_client_runner)
        assert "no redirect rules configured" in auto_output.lower()
        assert auto_records == []

        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_no_redirect_rule(routing, REDIRECT_CIDR)
        _assert_no_domain_redirect_rule(routing, REDIRECT_DOMAIN)
        _assert_routing_rule(routing, PRIMARY_HOST)

        state = _read_remote_json(client_host, CLIENT_STATE_FILE)
        assert not state.get("redirects")
        remaining_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert remaining_hosts == {PRIMARY_HOST}

        missing_remove = xp2p_client_runner(
            "client",
            "remove",
            SECONDARY_HOST,
            check=False,
        )
        assert missing_remove.rc != 0
        assert f'client endpoint "{SECONDARY_HOST}" not found' in _combined_output(missing_remove)

        xp2p_client_runner(
            "client",
            "remove",
            "--all",
            check=True,
        )

        final_output, records = _list_redirects(xp2p_client_runner)
        assert "no redirect rules configured" in final_output.lower()
        assert records == []
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
