from __future__ import annotations

import pytest

from tests.host.linux.flows import tunnel_b_to_a as flow

pytestmark = [pytest.mark.host, pytest.mark.linux]
tunnel_environment = flow.tunnel_environment


def test_forward_tunnel_operational(tunnel_environment):
    flow.assert_forward_tunnel_operational(tunnel_environment)


def test_client_and_server_redirect_with_nat(tunnel_environment):
    flow.assert_client_and_server_redirect_with_nat(tunnel_environment)


def test_reverse_redirect_via_server_portal(tunnel_environment):
    flow.assert_reverse_redirect_via_server_portal(tunnel_environment)

