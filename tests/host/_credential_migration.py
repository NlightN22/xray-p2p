from __future__ import annotations

from collections.abc import Callable
import json
import time


def connection_link(output: str) -> str:
    document = json.loads(output)
    result = document.get("result") or {}
    link = result.get("link")
    if not link:
        link = (result.get("credential") or {}).get("link")
    if isinstance(link, str) and link.startswith(("trojan://", "vless://")):
        return link
    raise AssertionError("Server user command did not emit a connection link")


def client_endpoint(config: dict, user: str) -> dict:
    for endpoint in config.get("endpoints") or []:
        if endpoint.get("user") == user:
            return endpoint
    raise AssertionError(f"Client endpoint for {user} is missing")


def runtime_endpoint(runtime: dict, user: str) -> dict:
    desired = runtime.get("desired") or {}
    for endpoint in desired.get("endpoints") or []:
        if endpoint.get("user") == user:
            return endpoint
    raise AssertionError(f"Live runtime endpoint for {user} is missing")


def server_user(config: dict, user: str) -> dict:
    for entry in config.get("users") or []:
        if entry.get("user_label") == user:
            return entry
    raise AssertionError(f"Server user {user} is missing")


def assert_client_persisted_credential_converged(
    desired: dict, runtime: dict, live_xray_artifact: dict, user: str, expected: str
) -> None:
    actual = {
        "Desired": str(client_endpoint(desired, user).get("password") or ""),
        "Live runtime": str(runtime_endpoint(runtime, user).get("credential") or ""),
        "Live Xray artifact": xray_outbound_credential(live_xray_artifact, user),
    }
    assert all(value == expected for value in actual.values()), (
        f"Credential state diverged for {user}: {actual}; expected {expected}"
    )


def xray_outbound_credential(xray: dict, user: str) -> str:
    for outbound in xray.get("outbounds") or []:
        settings = outbound.get("settings") or {}
        for server in settings.get("servers") or []:
            if server.get("email") == user:
                return str(server.get("password") or server.get("id") or "")
    raise AssertionError(f"Xray outbound for {user} is missing")


def xray_inbound_credential(xray: dict, user: str) -> str:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("protocol") != "trojan":
            continue
        for client in inbound.get("settings", {}).get("clients") or []:
            if client.get("email") == user:
                return str(client.get("password") or "")
    raise AssertionError(f"Xray inbound credential for {user} is missing")


def wait_until(
    read: Callable[[], object],
    accept: Callable[[object], bool],
    *,
    timeout: float,
    description: str,
) -> object:
    deadline = time.time() + timeout
    last: object = None
    while time.time() < deadline:
        last = read()
        if accept(last):
            return last
        time.sleep(1.0)
    raise AssertionError(f"Timed out waiting for {description}. Last state: {last}")
