from __future__ import annotations

from tests.host.linux.flows.tunnel_b_to_a_fixture import (
    CLIENT_DIAGNOSTICS_PORT,
    CLIENT_FORWARD_PORT,
    CLIENT_IP,
    CLIENT_REDIRECT_CIDR,
    CLIENT_REVERSE_TEST_IP,
    SERVER_DIAGNOSTICS_PORT,
    SERVER_FORWARD_PORT,
    SERVER_IP,
    SERVER_REDIRECT_CIDR,
    tunnel_environment,
)
from tests.host.linux.flows.tunnel_b_to_a_redirect_cleanup import assert_server_redirect_cleanup_on_user_remove
from tests.host.linux.flows.tunnel_b_to_a_forward import assert_forward_tunnel_operational
from tests.host.linux.flows.tunnel_b_to_a_redirect_nat import assert_client_and_server_redirect_with_nat
from tests.host.linux.flows.tunnel_b_to_a_reverse import assert_reverse_redirect_via_server_portal

__all__ = [
    "CLIENT_DIAGNOSTICS_PORT",
    "CLIENT_FORWARD_PORT",
    "CLIENT_IP",
    "CLIENT_REDIRECT_CIDR",
    "CLIENT_REVERSE_TEST_IP",
    "SERVER_DIAGNOSTICS_PORT",
    "SERVER_FORWARD_PORT",
    "SERVER_IP",
    "SERVER_REDIRECT_CIDR",
    "assert_client_and_server_redirect_with_nat",
    "assert_forward_tunnel_operational",
    "assert_reverse_redirect_via_server_portal",
    "assert_server_redirect_cleanup_on_user_remove",
    "tunnel_environment",
]

