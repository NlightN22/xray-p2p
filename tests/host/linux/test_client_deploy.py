from __future__ import annotations

import pytest

from tests.host.linux.flows import client_deploy_impl as impl

pytestmark = [pytest.mark.host, pytest.mark.linux]


def test_client_deploy_end_to_end(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    impl.test_client_deploy_end_to_end(client_host, server_host, xp2p_client_runner, xp2p_server_runner)


def test_client_deploy_end_to_end_proxy_mode(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    impl.test_client_deploy_end_to_end_proxy_mode(client_host, server_host, xp2p_client_runner, xp2p_server_runner)


def test_client_link_readds_removed_server_user(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    impl.test_client_link_readds_removed_server_user(client_host, server_host, xp2p_client_runner, xp2p_server_runner)


def test_server_deploy_falls_back_to_self_signed_on_invalid_cert(
    client_host, server_host, xp2p_client_runner, xp2p_server_runner
):
    impl.test_server_deploy_falls_back_to_self_signed_on_invalid_cert(
        client_host, server_host, xp2p_client_runner, xp2p_server_runner
    )


def test_deploy_tun_with_multiple_reverse_redirects(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    impl.test_deploy_tun_with_multiple_reverse_redirects(
        client_host, server_host, xp2p_client_runner, xp2p_server_runner
    )

