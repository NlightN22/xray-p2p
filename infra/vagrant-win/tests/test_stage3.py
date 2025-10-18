import pytest

from .helpers import check_iperf_open, ensure_stage_one, get_interface_ipv4


@pytest.fixture(scope="session")
def reverse_targets(host_r2, host_r3, host_c2, host_c3):
    ensure_stage_one(host_r2, "r2-client", "10.0.102.0/24")
    ensure_stage_one(host_r3, "r3-client", "10.0.103.0/24")

    c2_ip = get_interface_ipv4(host_c2, "eth1")
    c3_ip = get_interface_ipv4(host_c3, "eth1")

    return {"c2": c2_ip, "c3": c3_ip}


@pytest.mark.parametrize(
    "host_fixture,label,target_key",
    [
        ("host_r1", "r1 to c2 reverse tunnel", "c2"),
        ("host_r1", "r1 to c3 reverse tunnel", "c3"),
        ("host_c1", "c1 to c2 reverse tunnel", "c2"),
        ("host_c1", "c1 to c3 reverse tunnel", "c3"),
    ],
    ids=["router-r1-c2", "router-r1-c3", "client-c1-c2", "client-c1-c3"],
)
def test_reverse_tunnels_exit_on_clients(
    host_fixture,
    label,
    target_key,
    request,
    reverse_targets,
):
    host = request.getfixturevalue(host_fixture)
    target_ip = reverse_targets[target_key]
    check_iperf_open(host, label, target_ip)
