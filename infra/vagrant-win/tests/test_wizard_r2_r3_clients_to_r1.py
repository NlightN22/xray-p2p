import pytest

from .helpers import (
    check_iperf_open,
    client_is_installed,
    client_remove,
    ensure_stage_one,
    get_inbound_client_emails,
    get_interface_ipv4,
    load_clients_registry,
    load_inbounds_config,
    server_is_installed,
    server_remove,
)


@pytest.fixture(scope="class")
def wizard_context(host_r1, host_r2, host_r3, host_c1, host_c2, host_c3):
    routers = {
        "r2-client": (host_r2, "10.0.102.0/24"),
        "r3-client": (host_r3, "10.0.103.0/24"),
    }
    return {
        "server": host_r1,
        "routers": routers,
        "host_c1": host_c1,
        "host_c2": host_c2,
        "host_c3": host_c3,
    }


@pytest.mark.incremental
class TestWizardTwoClientsToR1:
    def test_stage_01_initial_cleanup(self, wizard_context):
        _reset_wizard_state(wizard_context["server"], wizard_context["routers"])

    def test_stage_02_preflight(self, wizard_context):
        _preflight_checks(wizard_context["routers"])

    def test_stage_03_provision(self, wizard_context):
        _provision_clients(wizard_context["server"], wizard_context["routers"])

    def test_stage_04_server_config(self, wizard_context):
        _validate_server_state(
            wizard_context["server"],
            expected_users=set(wizard_context["routers"].keys()),
        )

    def test_stage_05_direct_tunnels(self, wizard_context):
        _check_direct_tunnels(
            wizard_context["host_c1"],
            wizard_context["routers"]["r2-client"][0],
            wizard_context["routers"]["r3-client"][0],
            wizard_context["host_c2"],
            wizard_context["host_c3"],
        )

    def test_stage_06_reverse_tunnels(self, wizard_context):
        _check_reverse_tunnels(
            wizard_context["server"],
            wizard_context["host_c1"],
            wizard_context["host_c2"],
            wizard_context["host_c3"],
        )


def _reset_wizard_state(server_host, routers):
    server_remove(server_host, purge_core=True, check=False)
    assert not server_is_installed(server_host), "Server should be absent after cleanup"

    for user, (router_host, _) in routers.items():
        client_remove(router_host, purge_core=True, check=False)
        assert not client_is_installed(router_host), f"{user} client should be absent after cleanup"


def _preflight_checks(routers):
    for user, (router_host, _) in routers.items():
        check_iperf_open(router_host, f"{user} preflight", "10.0.0.1")


def _provision_clients(server_host, routers):
    for user, (router_host, client_lan) in routers.items():
        ensure_stage_one(router_host, user, client_lan)

    assert server_is_installed(server_host), "Server should be installed after provisioning"


def _validate_server_state(server_host, expected_users):
    inbounds = load_inbounds_config(server_host)
    inbound_emails = set(get_inbound_client_emails(inbounds))
    missing_inbound = expected_users - inbound_emails
    assert not missing_inbound, f"Missing clients in inbounds.json: {sorted(missing_inbound)}"

    clients = load_clients_registry(server_host)
    registry_emails = {entry.get("email") for entry in clients if isinstance(entry, dict)}
    missing_registry = expected_users - registry_emails
    assert not missing_registry, f"Missing clients in clients.json: {sorted(missing_registry)}"


def _check_direct_tunnels(host_c1, host_r2, host_r3, host_c2, host_c3):
    c1_ip = get_interface_ipv4(host_c1, "eth1")
    assert c1_ip, "Unable to determine c1 LAN IP"

    scenarios = [
        (host_r2, "router r2 direct tunnel"),
        (host_r3, "router r3 direct tunnel"),
        (host_c2, "client c2 direct tunnel"),
        (host_c3, "client c3 direct tunnel"),
    ]

    for host, label in scenarios:
        check_iperf_open(host, label, c1_ip)


def _check_reverse_tunnels(host_r1, host_c1, host_c2, host_c3):
    reverse_targets = {
        "c2": get_interface_ipv4(host_c2, "eth1"),
        "c3": get_interface_ipv4(host_c3, "eth1"),
    }

    for key, ip in reverse_targets.items():
        assert ip, f"Unable to determine {key} LAN IP"

    scenarios = [
        (host_r1, "r1 to c2 reverse tunnel", "c2"),
        (host_r1, "r1 to c3 reverse tunnel", "c3"),
        (host_c1, "c1 to c2 reverse tunnel", "c2"),
        (host_c1, "c1 to c3 reverse tunnel", "c3"),
    ]

    for host, label, target_key in scenarios:
        check_iperf_open(host, label, reverse_targets[target_key])
