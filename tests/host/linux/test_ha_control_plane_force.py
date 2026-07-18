from __future__ import annotations

import json

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _sessions
from tests.host.linux import env as linux_env
from tests.host.linux import _ha_control_plane_helpers as ha_helpers


@pytest.mark.host
@pytest.mark.linux
def test_ha_force_reconfiguration_allows_two_voter_emergency_recovery(linux_host_factory):
    active_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    lost_host = linux_host_factory(linux_env.DEFAULT_CLIENT)

    active = ha_helpers.runner(active_host)
    lost = ha_helpers.runner(lost_host)

    active_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    lost_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_CLIENT]

    ha_helpers.server_install(active, active_ip, "62201")
    ha_helpers.server_install(lost, lost_ip, "62202")

    ha_helpers.ha(active, "group", "create", ha_helpers.GROUP_ID, ha_helpers.GROUP_TAG)
    ha_helpers.ha(active, "member", "add", "active", "active-endpoint", active_ip, "62201", "trojan-tls")
    ha_helpers.ha(active, "peer", "self", "active")
    ha_helpers.ha(active, "peer", "add", "lost", ha_helpers.control_endpoint(lost_ip), ha_helpers.SECRET, "--allow-insecure")

    blocked = ha_helpers.ha(
        active,
        "member",
        "add",
        "blocked",
        "blocked-endpoint",
        lost_ip,
        "62202",
        "trojan-tls",
        check=False,
    )
    assert blocked.rc != 0
    assert "quorum is unavailable" in f"{blocked.stdout}\n{blocked.stderr}".lower()
    generation = ha_helpers.generation(active_host)
    assert int(generation.get("number") or 0) == 2
    assert "blocked" not in json.dumps(generation)

    ha_helpers.ha(
        active,
        "member",
        "add",
        "forced",
        "forced-endpoint",
        lost_ip,
        "62203",
        "trojan-tls",
        "--force",
        "--reason",
        "lost peer is permanently unavailable",
    )
    ha_helpers.assert_generation_member(active_host, "forced")


@pytest.mark.host
@pytest.mark.linux
def test_ha_force_reconfiguration_rejects_returning_stale_peer(linux_host_factory):
    active_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    stale_host = linux_host_factory(linux_env.DEFAULT_CLIENT)

    active = ha_helpers.runner(active_host)
    stale = ha_helpers.runner(stale_host)

    active_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    stale_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_CLIENT]

    ha_helpers.server_install(active, active_ip, "62211")
    ha_helpers.server_install(stale, stale_ip, "62212")

    ha_helpers.ha(active, "group", "create", ha_helpers.GROUP_ID, ha_helpers.GROUP_TAG)
    ha_helpers.ha(active, "member", "add", "active", "active-endpoint", active_ip, "62211", "trojan-tls")
    ha_helpers.ha(active, "member", "add", "stale", "stale-endpoint", stale_ip, "62212", "trojan-tls")
    ha_helpers.ha(active, "peer", "self", "active")
    ha_helpers.ha(active, "peer", "add", "stale", ha_helpers.control_endpoint(stale_ip), ha_helpers.SECRET, "--allow-insecure")
    ha_helpers.ha(stale, "peer", "self", "stale")
    ha_helpers.ha(stale, "peer", "add", "active", ha_helpers.control_endpoint(active_ip), ha_helpers.SECRET, "--allow-insecure")

    with (
        _sessions.xp2p_run_session(active_host, "server", helpers.INSTALL_ROOT, helpers.SERVER_CONFIG_DIR_NAME),
        _sessions.xp2p_run_session(stale_host, "server", helpers.INSTALL_ROOT, helpers.SERVER_CONFIG_DIR_NAME),
    ):
        ha_helpers.ha(active, "sync")
        synced_generation = ha_helpers.generation_number(active_host)
        assert ha_helpers.generation_number(stale_host) == synced_generation

    ha_helpers.ha(
        active,
        "member",
        "add",
        "replacement",
        "replacement-endpoint",
        stale_ip,
        "62213",
        "trojan-tls",
        "--force",
        "--reason",
        "stale peer is permanently unavailable",
    )
    forced_generation = ha_helpers.generation_number(active_host)
    assert forced_generation > synced_generation
    ha_helpers.assert_generation_member(active_host, "replacement")

    with _sessions.xp2p_run_session(active_host, "server", helpers.INSTALL_ROOT, helpers.SERVER_CONFIG_DIR_NAME):
        result = ha_helpers.ha(stale, "sync", check=False)

    output = f"{result.stdout}\n{result.stderr}".lower()
    assert result.rc != 0
    assert "generation is not newer" in output or "quorum is unavailable" in output
    assert ha_helpers.generation_number(active_host) == forced_generation
    ha_helpers.assert_generation_member(active_host, "replacement")
    assert "stale-change" not in json.dumps(ha_helpers.generation(active_host))
