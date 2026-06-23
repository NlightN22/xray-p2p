from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _identity_ldap
from tests.host.linux import _server_user_helpers as user_helpers
from tests.host.linux.test_identity_sync_e2e import (
    SERVER_HOST,
    _append_server_config,
    _identity_state,
    _ldap_provider_config,
    _redirect_rule,
)
from tests.host.linux.test_identity_provider_expansion import _add_group_redirect, _provision_identity


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_partial_group_removal_keeps_redirect_for_remaining_member(
    server_host, aux_host, xp2p_server_runner, ldap_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62058", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)

    state = _identity_state(server_host)
    alice_label = state["current"]["subjects"]["usr-10001"]["user_label"]
    carol_label = state["current"]["subjects"]["usr-10003"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)
    _provision_identity(xp2p_server_runner, carol_label)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-partial-membership.internal", reverse_tag, "engineering")
    rule = _redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "ldap-partial-membership.internal",
        reverse_tag,
    )
    assert sorted(rule.get("user") or []) == sorted([alice_label, carol_label])

    _identity_ldap.run_ldap(aux_host, "set-membership", "alice", "engineering", "absent")
    xp2p_server_runner("server", "identity", "sync", check=True)

    current = _identity_state(server_host)["current"]
    assert current["subjects"]["usr-10001"]["provisioned"] is True
    assert current["subjects"]["usr-10003"]["provisioned"] is True
    assert current["groups"]["engineering"].get("direct_members", []) == ["usr-10003"]
    rule = _redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "ldap-partial-membership.internal",
        reverse_tag,
    )
    assert rule.get("user") == [carol_label]


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_removed_group_selector_keeps_redirect_for_remaining_group(
    server_host, aux_host, xp2p_server_runner, ldap_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62059", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)

    state = _identity_state(server_host)
    alice_label = state["current"]["subjects"]["usr-10001"]["user_label"]
    carol_label = state["current"]["subjects"]["usr-10003"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)
    _provision_identity(xp2p_server_runner, carol_label)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    domain = "ldap-partial-group.internal"
    _add_group_redirect(xp2p_server_runner, domain, reverse_tag, "engineering")
    _add_redirect_group(xp2p_server_runner, domain, reverse_tag, "admins")
    rule = _redirect_rule(helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True), domain, reverse_tag)
    assert sorted(rule.get("user") or []) == sorted([alice_label, carol_label])

    desired_before = helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE)
    _identity_ldap.run_ldap(aux_host, "remove-group", "engineering")
    xp2p_server_runner("server", "identity", "sync", check=True)

    current = _identity_state(server_host)["current"]
    assert "engineering" not in current["groups"]
    assert current["groups"]["admins"].get("direct_members", []) == ["usr-10001"]
    assert helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE) == desired_before
    rule = _redirect_rule(helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True), domain, reverse_tag)
    assert rule.get("user") == [alice_label]


def _add_redirect_group(xp2p_server_runner, domain: str, tag: str, group: str) -> None:
    xp2p_server_runner(
        "server",
        "redirect",
        "access",
        "add-group",
        "--domain",
        domain,
        "--tag",
        tag,
        "--allow-group",
        group,
        check=True,
    )
