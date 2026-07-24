from __future__ import annotations

import json
import re
import time
from typing import Iterable

ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")


def strip_ansi(value: str | None) -> str:
    if not value:
        return ""
    return ANSI_ESCAPE_RE.sub("", value)


def parse_state_result(output: str) -> list[dict[str, str]]:
    document = json.loads(output)
    tunnels = (document.get("result") or {}).get("tunnels") or []
    rows: list[dict[str, str]] = []
    for tunnel in tunnels:
        status = str(tunnel.get("status") or "")
        if not status:
            status = "alive" if tunnel.get("alive") else "stale"
        rows.append(
            {
                "TAG": str(tunnel.get("tag") or ""),
                "HOST": str(tunnel.get("host") or ""),
                "STATUS": status,
                "MODE": str(tunnel.get("mode") or ""),
                "CHECK": str(tunnel.get("capability") or ""),
                "LAST_ATTEMPT": str(tunnel.get("last_seen") or ""),
                "LAST_SUCCESS": str(tunnel.get("last_success") or ""),
                "FAILURE_STAGE": str(tunnel.get("failure_stage") or ""),
                "LAST_RTT": f"{int(tunnel.get('last_rtt_millis') or 0)}ms",
                "AVG_RTT": f"{float(tunnel.get('average_rtt_millis') or 0):.1f}ms",
                "LAST_UPDATE": str(tunnel.get("last_seen") or ""),
                "CLIENT_USER": str(tunnel.get("user") or ""),
                "CLIENT_IP": str(tunnel.get("client_ip") or ""),
            }
        )
    return rows


def forward_entry_for_target(entries: Iterable[dict], target_host: str, target_port: int) -> dict:
    normalized_host = target_host.strip()
    normalized_port = int(target_port)
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_host = (entry.get("target_host") or "").strip()
        recorded_port = int(entry.get("target_port") or entry.get("targetPort") or 0)
        if recorded_host == normalized_host and recorded_port == normalized_port:
            return entry
    raise AssertionError(f"Forward entry targeting {target_host}:{target_port} not found in state")


def listen_port_from_entry(entry: dict) -> int:
    port = int(entry.get("listen_port") or entry.get("listenPort") or 0)
    if port <= 0:
        raise AssertionError("Forward entry is missing listen port")
    return port


def assert_zero_loss(ping_result, context: str) -> None:
    stdout = (ping_result.stdout or "").lower()
    assert "0% loss" in stdout, (
        f"xp2p ping {context} did not report full delivery:\n"
        f"{ping_result.stdout}"
    )


def wait_for_alive_entry(
    runner,
    role: str,
    install_path: str,
    expected_tag: str,
    expected_host: str,
    expected_user: str,
    expected_client_ip: str,
    *,
    timeout_seconds: float = 60.0,
    poll_interval: float = 2.0,
) -> dict:
    deadline = time.time() + timeout_seconds
    last_stdout = ""
    while time.time() < deadline:
        result = runner(
            role,
            "state",
            "--json",
            "--path",
            install_path,
            check=True,
        )
        last_stdout = result.stdout or ""
        for row in parse_state_result(last_stdout):
            tag = row.get("TAG", "").strip()
            host_value = row.get("HOST", "").strip()
            status = row.get("STATUS", "").strip().lower()
            if tag != expected_tag or host_value != expected_host or status not in {"alive", "healthy"}:
                continue
            client_user = row.get("CLIENT_USER", "").strip()
            client_ip = row.get("CLIENT_IP", "").strip()
            if client_user != expected_user:
                raise AssertionError(
                    f"Heartbeat CLIENT_USER mismatch (expected {expected_user}, got {client_user})"
                )
            if client_ip != expected_client_ip:
                raise AssertionError(
                    f"Heartbeat CLIENT_IP mismatch (expected {expected_client_ip}, got {client_ip})"
                )
            return row
        time.sleep(poll_interval)
    raise AssertionError(
        "Alive heartbeat entry not observed for "
        f"{expected_tag}@{expected_host}. Last xp2p {role} state output:\n{last_stdout}"
    )
