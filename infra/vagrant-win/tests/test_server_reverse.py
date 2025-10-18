import json
from uuid import uuid4

import pytest

from .helpers import (
    ensure_stage_one,
    get_reverse_portals,
    get_routing_rules,
    load_reverse_tunnels,
    load_routing_config,
    server_reverse_add,
    server_reverse_remove,
    server_reverse_remove_raw,
)


@pytest.fixture(scope="module", autouse=True)
def ensure_stage_one_prereq(host_r2, host_r3):
    ensure_stage_one(host_r2, "r2-client", "10.0.102.0/24")
    ensure_stage_one(host_r3, "r3-client", "10.0.103.0/24")


def _unique_username() -> str:
    return f"pytest-{uuid4().hex[:12]}"


def _unique_subnet() -> str:
    suffix = int(uuid4().hex[:2], 16)
    return f"10.200.{suffix}.0/24"


def test_server_reverse_add_records_tunnel(host_r1):
    username = _unique_username()
    subnet = _unique_subnet()

    routing_before = load_routing_config(host_r1)
    tunnels_before = load_reverse_tunnels(host_r1)

    added = False
    try:
        add_result = server_reverse_add(host_r1, username, [subnet])
        added = True
        combined_output = f"{add_result.stdout}\n{add_result.stderr}".lower()
        assert "recorded" in combined_output, "Add command did not confirm recording"

        tunnels_after = load_reverse_tunnels(host_r1)
        assert len(tunnels_after) == len(tunnels_before) + 1, "Tunnel count not increased"
        tunnel_entry = next(
            (item for item in tunnels_after if item.get("username") == username),
            None,
        )
        assert tunnel_entry, "Tunnel entry missing in tunnels.json"
        assert subnet in tunnel_entry.get("subnets", []), "Subnet not recorded"

        routing_after = load_routing_config(host_r1)
        domain = f"{username}.rev"
        portals = get_reverse_portals(routing_after)
        assert any(portal.get("domain") == domain for portal in portals), "Portal not registered"

        rules = get_routing_rules(routing_after)
        assert any(
            any(entry.endswith(domain) for entry in (rule.get("domain") or []))
            for rule in rules
        ), "Routing rules missing domain entry"
    finally:
        if added:
            server_reverse_remove(host_r1, username)

    routing_final = load_routing_config(host_r1)
    tunnels_final = load_reverse_tunnels(host_r1)
    assert json.dumps(routing_final, sort_keys=True) == json.dumps(
        routing_before, sort_keys=True
    ), "Routing config did not revert to original state"
    assert json.dumps(tunnels_final, sort_keys=True) == json.dumps(
        tunnels_before, sort_keys=True
    ), "Tunnels config did not revert to original state"


def test_server_reverse_remove_missing_tunnel(host_r1):
    username = _unique_username()
    result = server_reverse_remove_raw(host_r1, username)
    assert result.rc != 0, "Expected failure when removing unknown tunnel"
    combined_output = f"{result.stdout}\n{result.stderr}".lower()
    assert "not found" in combined_output, "Missing tunnel error message expected"
