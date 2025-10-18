import pytest

from .helpers import check_iperf_open, ensure_stage_one


@pytest.mark.parametrize(
    "host_fixture,user,client_lan",
    [
        ("host_r2", "r2-client", "10.0.102.0/24"),
        ("host_r3", "r3-client", "10.0.103.0/24"),
    ],
    ids=["router-r2-client", "router-r3-client"],
)
def test_stage_one_provisions_tunnel(host_fixture, user, client_lan, request):
    router = request.getfixturevalue(host_fixture)

    # Pre-check: routers must reach r1 directly before provisioning the tunnel.
    check_iperf_open(router, f"{user} precheck", "10.0.0.1")

    # Provision tunnel via xsetup wizard (idempotent across tests).
    ensure_stage_one(router, user, client_lan)

    # Post-check: traffic to r1 LAN must succeed through the tunnel.
    check_iperf_open(router, f"{user} post-setup", "10.0.101.1")
