from __future__ import annotations

import re
import time
from typing import Iterable

STATE_TABLE_HEADER = (
    "TAG",
    "HOST",
    "STATUS",
    "LAST_RTT",
    "AVG_RTT",
    "LAST_UPDATE",
    "CLIENT_USER",
    "CLIENT_IP",
)

ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")


def strip_ansi(value: str | None) -> str:
    if not value:
        return ""
    return ANSI_ESCAPE_RE.sub("", value)


def split_state_line(line: str) -> list[str]:
    parts = [segment.strip() for segment in line.split("\t") if segment.strip()]
    if len(parts) >= len(STATE_TABLE_HEADER):
        return parts[: len(STATE_TABLE_HEADER)]
    regex_parts = [segment.strip() for segment in re.split(r"\s{2,}", line) if segment.strip()]
    if len(regex_parts) >= len(STATE_TABLE_HEADER):
        return regex_parts[: len(STATE_TABLE_HEADER)]
    return regex_parts or parts


def parse_state_rows(output: str) -> list[dict[str, str]]:
    cleaned = strip_ansi(output)
    header = None
    rows: list[dict[str, str]] = []
    for raw_line in cleaned.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        cells = split_state_line(line)
        if not cells:
            continue
        if tuple(cells[: len(STATE_TABLE_HEADER)]) == STATE_TABLE_HEADER:
            header = list(STATE_TABLE_HEADER)
            continue
        if not header:
            continue
        if len(cells) != len(header):
            continue
        if all(cell.strip() == "-" for cell in cells):
            continue
        rows.append({header[idx]: cell.strip() for idx, cell in enumerate(cells)})
    return rows


def forward_entry_for_target(entries: Iterable[dict], target_ip: str, target_port: int) -> dict:
    normalized_ip = target_ip.strip()
    normalized_port = int(target_port)
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_ip = (entry.get("target_ip") or entry.get("targetIP") or "").strip()
        recorded_port = int(entry.get("target_port") or entry.get("targetPort") or 0)
        if recorded_ip == normalized_ip and recorded_port == normalized_port:
            return entry
    raise AssertionError(f"Forward entry targeting {target_ip}:{target_port} not found in state")


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


def ping_with_retries(runner, args: tuple[str, ...], context: str, attempts: int = 3, delay_seconds: float = 2.0):
    last_result = None
    for attempt in range(1, attempts + 1):
        result = runner(*args, check=False)
        if result.rc == 0:
            return result
        last_result = result
        if attempt < attempts:
            time.sleep(delay_seconds)
    assert last_result is not None, "xp2p ping failed but no result captured"
    raise AssertionError(
        f"xp2p ping {context} failed after {attempts} attempts "
        f"(exit {last_result.rc}).\nSTDOUT:\n{last_result.stdout}\nSTDERR:\n{last_result.stderr}"
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
            "--path",
            install_path,
            check=True,
        )
        last_stdout = result.stdout or ""
        for row in parse_state_rows(last_stdout):
            tag = row.get("TAG", "").strip()
            host_value = row.get("HOST", "").strip()
            status = row.get("STATUS", "").strip().lower()
            if tag != expected_tag or host_value != expected_host or status != "alive":
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
