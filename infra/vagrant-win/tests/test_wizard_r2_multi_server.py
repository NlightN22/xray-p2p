import pytest

from .helpers import (
    check_iperf_open,
    client_is_installed,
    client_remove,
    ensure_stage_one,
    run_checked,
    server_is_installed,
    server_remove,
)


@pytest.fixture(scope="class")
def wizard_context(host_r1, host_r2, host_r3, host_c1, host_c2, host_c3):
    targets = {
        "r1": {
            "server": host_r1,
            "user": "r2-r1",
            "server_addr": "10.0.0.1",
            "server_lan": "10.0.101.0/24",
            "client": host_c1,
        },
        "r3": {
            "server": host_r3,
            "user": "r2-r3",
            "server_addr": "10.0.0.3",
            "server_lan": "10.0.103.0/24",
            "client": host_c3,
        },
    }
    return {
        "controller": host_r2,
        "client_lan": "10.0.102.0/24",
        "targets": targets,
        "reverse_hosts": {
            "r2": host_r2,
            "c2": host_c2,
        },
    }


@pytest.mark.incremental
class TestWizardMultiServerProvisioning:
    def test_stage_01_initial_cleanup(self, wizard_context):
        _reset_state(wizard_context)

    def test_stage_02_install_r1_from_r2(self, wizard_context):
        _provision_server_via_wizard(wizard_context, "r1", skip_if_active=False)

    def test_stage_03_direct_tunnels_r1(self, wizard_context):
        _check_direct_tunnels(wizard_context, "r1")

    def test_stage_04_install_r3_from_r2(self, wizard_context):
        _provision_server_via_wizard(wizard_context, "r3", skip_if_active=False)
        assert server_is_installed(
            wizard_context["targets"]["r1"]["server"]
        ), "r1 server should remain installed after provisioning r3"

    def test_stage_05_direct_tunnels_r3(self, wizard_context):
        _check_direct_tunnels(wizard_context, "r3")

    def test_stage_06_reverse_tunnels(self, wizard_context):
        _check_reverse_tunnels(wizard_context)


def _reset_state(context: dict):
    controller = context["controller"]
    client_remove(controller, purge_core=True, check=False)
    server_remove(controller, purge_core=True, check=False)
    assert not client_is_installed(controller), "Controller client should be absent after cleanup"

    for key, target in context["targets"].items():
        server_remove(target["server"], purge_core=True, check=False)
        client_remove(target["server"], purge_core=True, check=False)
        assert not server_is_installed(target["server"]), f"{key} server should be absent after cleanup"


def _provision_server_via_wizard(context: dict, target_key: str, *, skip_if_active: bool):
    controller = context["controller"]
    target = context["targets"][target_key]

    ensure_stage_one(
        controller,
        target["user"],
        context["client_lan"],
        server_addr=target["server_addr"],
        server_lan=target["server_lan"],
        skip_if_active=skip_if_active,
    )

    assert server_is_installed(
        target["server"]
    ), f"{target_key} server should report as installed after wizard run"
    assert client_is_installed(
        controller
    ), "Controller client should remain installed after wizard run"


def _check_direct_tunnels(context: dict, target_key: str):
    controller = context["controller"]
    target = context["targets"][target_key]
    server_ip = _get_lan_ipv4(
        target["server"],
        target["server_lan"],
        description=f"{target_key} router",
    )
    client_ip = _get_lan_ipv4(
        target["client"],
        None,
        description=f"{target_key} client",
        fallback_interfaces=("eth1", "eth0", "br-lan", "lan"),
    )

    check_iperf_open(controller, f"r2 to {target_key} router tunnel", server_ip)
    check_iperf_open(controller, f"r2 to {target_key} client tunnel", client_ip)


def _check_reverse_tunnels(context: dict):
    controller_hosts = {}
    for name, host in context["reverse_hosts"].items():
        controller_hosts[name] = _get_lan_ipv4(
            host,
            context["client_lan"] if name == "r2" else None,
            description=name,
        )
    for name, ip in controller_hosts.items():
        assert ip, f"Unable to determine LAN IP for {name}"

    for target_key, target in context["targets"].items():
        server_host = target["server"]
        for name, ip in controller_hosts.items():
            check_iperf_open(server_host, f"{target_key} to {name} reverse tunnel", ip)


def _get_lan_ipv4(
    host,
    lan_cidr: str | None,
    *,
    description: str,
    fallback_interfaces: tuple[str, ...] = ("eth1", "br-lan", "lan", "eth0"),
) -> str:
    for iface in fallback_interfaces:
        result = host.run(f"ip -o -4 addr show dev {iface}")
        lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
        if lines:
            segment = lines[0].split()
            if len(segment) >= 4:
                return segment[3].split("/", 1)[0]

    result = run_checked(host, "ip -o -4 addr show", f"discover IPv4s for {description}")
    lan_prefix = None
    if lan_cidr:
        lan_prefix = lan_cidr.split("/", 1)[0].rsplit(".", 1)[0] + "."
    for line in result.stdout.splitlines():
        parts = line.split()
        if len(parts) < 4:
            continue
        candidate = parts[3].split("/", 1)[0].strip()
        if lan_prefix:
            if candidate.startswith(lan_prefix):
                return candidate
        elif not candidate.startswith("10.0.0."):
            return candidate

    raise AssertionError(f"Unable to determine LAN IPv4 for {description}.")
