import json
from uuid import uuid4

import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    ensure_stage_one,
    get_reverse_portals,
    get_routing_rules,
    load_reverse_tunnels,
    load_routing_config,
    server_install,
    server_is_installed,
    server_reverse_add,
    server_reverse_remove,
    server_reverse_remove_raw,
    server_remove,
)

SERVER_REVERSE_NAME = "pytest-reverse"


@pytest.fixture(scope="module", autouse=True)
def ensure_stage_one_prereq(host_r2, host_r3):
    ensure_stage_one(host_r2, "r2-client", "10.0.102.0/24")
    ensure_stage_one(host_r3, "r3-client", "10.0.103.0/24")


@pytest.fixture(autouse=True)
def cleanup_remote_tmp(host_r1):
    host_r1.run("rm -f /tmp/common*.sh /tmp/server_reverse.sh")
    host_r1.run("rm -rf /tmp/xray-p2p /tmp/scripts")
    yield


def _unique_username() -> str:
    return f"pytest-{uuid4().hex[:12]}"


def _unique_subnet() -> str:
    suffix = int(uuid4().hex[:2], 16)
    return f"10.200.{suffix}.0/24"


@pytest.mark.serial
def test_server_reverse_full_flow(host_r1):
    username = _unique_username()
    subnet = _unique_subnet()

    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1), "Server must be absent before provisioning"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Server config directory should be removed"
    assert load_reverse_tunnels(host_r1) == [], "Reverse tunnels must not persist after server removal"

    install_env = {
        "XRAY_REISSUE_CERT": "1",
        "XRAY_SKIP_PORT_CHECK": "1",
    }
    try:
        server_install(host_r1, SERVER_REVERSE_NAME, "9555", "--force", env=install_env)
        assert server_is_installed(host_r1), "Server should report as installed after provisioning"

        routing_before = load_routing_config(host_r1)
        tunnels_before = load_reverse_tunnels(host_r1)

        added = False
        try:
            add_result = server_reverse_add(
                host_r1,
                username,
                [subnet],
                server_id=SERVER_REVERSE_NAME,
            )
            added = True
            combined_output = f"{add_result.stdout}\n{add_result.stderr}".lower()
            assert "recorded" in combined_output, "Add command did not confirm recording"

            tunnels_after = load_reverse_tunnels(host_r1)
            assert len(tunnels_after) == len(tunnels_before) + 1, "Tunnel count not increased"
            new_entries = [entry for entry in tunnels_after if entry not in tunnels_before]
            assert new_entries, "New tunnel entry missing in tunnels.json"
            tunnel_entry = new_entries[0]
            assert subnet in tunnel_entry.get("subnets", []), "Subnet not recorded"
            assert (
                tunnel_entry.get("server_id") == SERVER_REVERSE_NAME
            ), "Server identifier not recorded"

            routing_after = load_routing_config(host_r1)
            domain = tunnel_entry.get("domain")
            assert isinstance(domain, str) and domain, "Domain not recorded for tunnel"
            portals = get_reverse_portals(routing_after)
            assert any(portal.get("domain") == domain for portal in portals), "Portal not registered"

            rules = get_routing_rules(routing_after)
            assert any(
                any(entry.endswith(domain) for entry in (rule.get("domain") or []))
                for rule in rules
            ), "Routing rules missing domain entry"
        finally:
            if added:
                server_reverse_remove(
                    host_r1,
                    username,
                    server_id=SERVER_REVERSE_NAME,
                )

        routing_final = load_routing_config(host_r1)
        tunnels_final = load_reverse_tunnels(host_r1)
        assert json.dumps(tunnels_final, sort_keys=True) == json.dumps(
            tunnels_before, sort_keys=True
        ), "Tunnels config did not revert to original state"
        assert get_reverse_portals(routing_final) == get_reverse_portals(
            routing_before
        ), "Reverse portals were not restored after tunnel removal"
        assert get_routing_rules(routing_final) == get_routing_rules(
            routing_before
        ), "Routing rules were not restored after tunnel removal"
        final_strategy = routing_final.get("routing", {}).get("domainStrategy")
        initial_strategy = routing_before.get("routing", {}).get("domainStrategy")
        if initial_strategy is None:
            assert final_strategy in (
                None,
                "AsIs",
            ), "Domain strategy should revert to default state"
        else:
            assert (
                final_strategy == initial_strategy
            ), "Domain strategy changed unexpectedly"
    finally:
        server_remove(host_r1, purge_core=True, check=False)

    assert not server_is_installed(host_r1), "Server should be removed after cleanup"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Cleanup should remove server config directory"


def test_server_reverse_remove_missing_tunnel(host_r1):
    username = _unique_username()
    result = server_reverse_remove_raw(
        host_r1,
        username,
        server_id=SERVER_REVERSE_NAME,
    )
    assert result.rc != 0, "Expected failure when removing unknown tunnel"
    combined_output = f"{result.stdout}\n{result.stderr}".lower()
    assert "not found" in combined_output, "Missing tunnel error message expected"
