import pytest

from .helpers import check_iperf_open, ensure_stage_one, get_interface_ipv4


@pytest.fixture(scope="session")
def c1_ipv4(host_c1):
    return get_interface_ipv4(host_c1, "eth1")


@pytest.mark.parametrize(
    "host_fixture,label,router_fixture,user,client_lan",
    [
        ("host_r2", "router r2 direct tunnel", "host_r2", "r2-client", "10.0.102.0/24"),
        ("host_r3", "router r3 direct tunnel", "host_r3", "r3-client", "10.0.103.0/24"),
        ("host_c2", "client c2 direct tunnel", "host_r2", "r2-client", "10.0.102.0/24"),
        ("host_c3", "client c3 direct tunnel", "host_r3", "r3-client", "10.0.103.0/24"),
    ],
    ids=["router-r2", "router-r3", "client-c2", "client-c3"],
)
def test_direct_tunnels_reach_c1(
    host_fixture,
    label,
    router_fixture,
    user,
    client_lan,
    request,
    c1_ipv4,
):
    router_host = request.getfixturevalue(router_fixture)
    ensure_stage_one(router_host, user, client_lan)

    host = request.getfixturevalue(host_fixture)
    check_iperf_open(host, label, c1_ipv4)
