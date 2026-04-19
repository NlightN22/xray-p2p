from __future__ import annotations

import pytest

from tests.host.openwrt.flows import tunnel_b_to_a_impl as flow

pytestmark = [pytest.mark.host, pytest.mark.linux]
tunnel_environment = flow.tunnel_environment


def test_forward_tunnel_operational(tunnel_environment):
    flow.run_forward_tunnel_operational(tunnel_environment)


def test_client_redirect_through_server(tunnel_environment):
    flow.run_client_redirect_through_server(tunnel_environment)


def test_reverse_redirect_via_server_portal(tunnel_environment):
    flow.run_reverse_redirect_via_server_portal(tunnel_environment)

