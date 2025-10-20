from __future__ import annotations

import json
import shlex
from typing import Any, Dict, List

from .constants import (
    SERVER_CLIENTS_PATH,
    SERVER_INBOUNDS_PATH,
    SERVER_ROUTING_PATH,
    SERVER_TUNNELS_PATH,
)


def _read_json_file(host, path: str, default: Any):
    quoted = shlex.quote(path)
    result = host.run(f"cat {quoted}")
    if result.rc != 0:
        return default
    content = result.stdout.strip()
    if not content:
        return default
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise AssertionError(
            f"Invalid JSON content at {path}.\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        ) from exc


def load_clients_registry(host) -> List[Dict[str, Any]]:
    data = _read_json_file(host, SERVER_CLIENTS_PATH, default=[])
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        clients = data.get("clients", [])
        if isinstance(clients, list):
            return clients
    return []


def load_inbounds_config(host) -> Dict[str, Any]:
    data = _read_json_file(host, SERVER_INBOUNDS_PATH, default={})
    if isinstance(data, dict):
        return data
    return {}


def get_inbound_client_emails(inbounds: Dict[str, Any]) -> List[str]:
    emails: List[str] = []
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        settings = inbound.get("settings", {})
        if not isinstance(settings, dict):
            continue
        for client in settings.get("clients", []):
            if isinstance(client, dict):
                email = client.get("email")
                if isinstance(email, str) and email:
                    emails.append(email)
    return emails


def load_routing_config(host) -> Dict[str, Any]:
    data = _read_json_file(host, SERVER_ROUTING_PATH, default={})
    if isinstance(data, dict):
        return data
    return {}


def load_reverse_tunnels(host) -> List[Dict[str, Any]]:
    data = _read_json_file(host, SERVER_TUNNELS_PATH, default=[])
    if isinstance(data, list):
        return data
    return []


def get_routing_rules(routing: Dict[str, Any]) -> List[Dict[str, Any]]:
    rules = routing.get("routing", {}).get("rules", [])
    return [rule for rule in rules if isinstance(rule, dict)]


def get_reverse_portals(routing: Dict[str, Any]) -> List[Dict[str, Any]]:
    portals = routing.get("reverse", {}).get("portals", [])
    return [portal for portal in portals if isinstance(portal, dict)]


__all__ = [
    "load_clients_registry",
    "load_inbounds_config",
    "get_inbound_client_emails",
    "load_routing_config",
    "load_reverse_tunnels",
    "get_routing_rules",
    "get_reverse_portals",
]
