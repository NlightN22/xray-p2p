from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux import env as linux_env
from tests.host.linux import _ha_control_plane_helpers as ha_helpers
from tests.host.linux.test_identity_sync_e2e import (
    _append_server_config,
    _identity_state,
    _labels_for_group,
    _ldap_provider_config,
    _redirect_rule,
)

HA_IDENTITY_DOMAIN = "ha-engineering.internal"
HA_IDENTITY_REVERSE_TAG = "ha-identity-portal.rev"


@pytest.mark.host
@pytest.mark.linux
def test_ha_identity_acl_replicates_to_backup_server(linux_host_factory, ldap_directory):
    primary_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    backup_host = linux_host_factory(linux_env.DEFAULT_AUX)

    primary = ha_helpers.runner(primary_host)
    backup = ha_helpers.runner(backup_host)
    primary_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_SERVER]
    backup_ip = ha_helpers.HOST_ONLY_IPS[linux_env.DEFAULT_AUX]

    ha_helpers.server_install(primary, primary_ip, "62201")
    ha_helpers.server_install(backup, backup_ip, "62202")
    _append_server_config(primary_host, _ldap_provider_config(max_cache_age="24h"))

    primary("server", "identity", "sync", check=True)
    state = _identity_state(primary_host)
    alice_label = state["current"]["subjects"]["usr-10001"]["user_label"]
    carol_label = state["current"]["subjects"]["usr-10003"]["user_label"]
    primary("server", "identity", "provision", alice_label, "--host", primary_ip, check=True)
    primary("server", "identity", "provision", carol_label, "--host", primary_ip, check=True)
    state = _identity_state(primary_host)
    engineering_labels = sorted(_labels_for_group(state["current"], "engineering"))

    ha_helpers.ha(primary, "group", "create", ha_helpers.CLIENT_GROUP_ID, ha_helpers.CLIENT_GROUP_TAG)
    ha_helpers.ha(primary, "member", "add", "primary", "ha-identity-primary", primary_ip, "62201", "trojan-tls")
    ha_helpers.ha(primary, "member", "add", "backup", "ha-identity-backup", backup_ip, "62202", "trojan-tls")
    ha_helpers.ha(primary, "channel", "create", "identity-portal", HA_IDENTITY_REVERSE_TAG, HA_IDENTITY_REVERSE_TAG)
    ha_helpers.ha(
        primary,
        "redirect",
        "add",
        "identity-portal",
        "--domain",
        HA_IDENTITY_DOMAIN,
        "--access",
        "restricted",
        "--allow-group",
        "engineering",
    )

    try:
        ha_helpers.ha(primary, "peer", "self", "primary")
        ha_helpers.ha(primary, "peer", "add", "backup", ha_helpers.control_endpoint(backup_ip), ha_helpers.SECRET, "--allow-insecure")
        ha_helpers.ha(backup, "peer", "self", "backup")
        ha_helpers.ha(backup, "peer", "add", "primary", ha_helpers.control_endpoint(primary_ip), ha_helpers.SECRET, "--allow-insecure")

        backup("--log-level", "debug", "server", "service", "start", check=True)
        runtime.wait_for_service(backup_host, "server", active=True)
        runtime.wait_for_live_xray(backup_host, "server")

        ha_helpers.ha(primary, "sync")
        runtime.wait_for_apply_clear(backup_host)
        ha_helpers.assert_generation_member(backup_host, "backup")

        backup_identity = _identity_state(backup_host)
        assert backup_identity["current"]["id"] == state["current"]["id"]
        backup_labels = sorted(_labels_for_group(backup_identity["current"], "engineering"))
        assert backup_labels == engineering_labels

        routing = helpers.render_xray(backup_host, backup, "server", desired=False)
        rule = _redirect_rule(routing, HA_IDENTITY_DOMAIN, HA_IDENTITY_REVERSE_TAG)
        assert sorted(rule.get("user") or []) == engineering_labels
    finally:
        runtime.stop_service(backup, "server")
