from __future__ import annotations

import pytest

from tests.host.linux.flows import tun_mode_full_impl as impl

pytestmark = [pytest.mark.host, pytest.mark.linux]


def test_client_tun_mode_full_tunnel_routes_and_dns(client_host, xp2p_client_runner):
    impl.test_client_tun_mode_full_tunnel_routes_and_dns(client_host, xp2p_client_runner)


def test_client_tun_mode_full_tunnel_routes_restore_after_purge(client_host, xp2p_client_runner):
    impl.test_client_tun_mode_full_tunnel_routes_restore_after_purge(client_host, xp2p_client_runner)


def test_client_tun_mode_full_tunnel_selection_and_prompt(client_host, xp2p_client_runner):
    impl.test_client_tun_mode_full_tunnel_selection_and_prompt(client_host, xp2p_client_runner)


def test_client_verbose_flags_available(client_host):
    impl.test_client_verbose_flags_available(client_host)


def test_client_redirect_default_route_rejected(client_host, xp2p_client_runner):
    impl.test_client_redirect_default_route_rejected(client_host, xp2p_client_runner)


def test_client_tun_mode_full_unresolved_endpoint_fails(client_host, xp2p_client_runner):
    impl.test_client_tun_mode_full_unresolved_endpoint_fails(client_host, xp2p_client_runner)

