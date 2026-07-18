from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _identity_keycloak
from tests.host.linux import _identity_ldap
from tests.host.linux import _server_user_helpers as user_helpers
from tests.host.linux.test_identity_provider_expansion import _add_group_redirect, _keycloak_token, _provision_identity
from tests.host.linux.test_identity_sync_e2e import (
    SERVER_HOST,
    _append_server_config,
    _combined,
    _identity_state,
    _keycloak_provider_config,
    _labels_for_group,
    _ldap_provider_config,
    _redirect_rule,
    _replace_server_config,
    _write_server_config,
)


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_authoritative_changes_cascade_readd_scope_and_new_group(
    server_host, aux_host, xp2p_server_runner, ldap_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62060", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)
    initial = _identity_state(server_host)["current"]
    alice_label = initial["subjects"]["usr-10001"]["user_label"]
    carol_label = initial["subjects"]["usr-10003"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)
    _provision_identity(xp2p_server_runner, carol_label)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    operator = "operator@example.com"
    xp2p_server_runner("server", "user", "add", "--id", operator, "--password", "operator-secret", "--host", SERVER_HOST, check=True)
    operator_tag = helpers.expected_reverse_tag(operator, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-owned-removal.internal", reverse_tag, "engineering")
    _add_group_redirect(xp2p_server_runner, "ldap-scope.internal", reverse_tag, "engineering")
    _add_explicit_user_redirect(xp2p_server_runner, "ldap-explicit.internal", operator_tag, alice_label, operator)
    assert alice_label in _server_user_labels(server_host, xp2p_server_runner)

    _identity_ldap.run_ldap(aux_host, "remove-user", "alice")
    xp2p_server_runner("server", "identity", "sync", check=True)
    removed = _identity_state(server_host)["current"]
    assert "usr-10001" not in removed["subjects"]
    assert alice_label not in _server_user_labels(server_host, xp2p_server_runner)
    assert reverse_tag not in _server_reverse(server_host)
    helpers.assert_no_domain_redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "ldap-owned-removal.internal",
        reverse_tag,
    )
    assert operator in _desired_redirect_users(server_host, "ldap-explicit.internal")

    _identity_ldap.run_ldap(aux_host, "add-user", "usr-11901", "Alice Example", "alice", "alice@example.test")
    _identity_ldap.run_ldap(aux_host, "set-membership", "alice", "engineering", "present")
    xp2p_server_runner("server", "identity", "sync", check=True)
    readded = _identity_state(server_host)["current"]
    new_label = readded["subjects"]["usr-11901"]["user_label"]
    assert new_label != alice_label
    assert readded["subjects"]["usr-11901"].get("provisioned") in (None, False)

    _identity_ldap.run_ldap(aux_host, "add-group", "20901", "support")
    _identity_ldap.run_ldap(aux_host, "set-membership", "carol", "support", "present")
    _replace_server_config(
        server_host,
        'group_ids = ["engineering", "admins"]',
        'group_ids = ["engineering", "admins", "support"]',
    )
    xp2p_server_runner("server", "identity", "sync", check=True)
    assert _identity_state(server_host)["current"]["groups"]["support"]["direct_members"] == ["usr-10003"]
    support_tag = helpers.expected_reverse_tag(carol_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-support.internal", support_tag, "support")
    assert _redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "ldap-support.internal",
        support_tag,
    ).get("user") == [carol_label]

    _replace_server_config(server_host, 'group_ids = ["engineering", "admins", "support"]', 'group_ids = ["admins"]')
    xp2p_server_runner("server", "identity", "sync", check=True)
    narrowed = _identity_state(server_host)["current"]
    assert "engineering" not in narrowed["groups"]
    assert "usr-10003" not in narrowed["subjects"]
    helpers.assert_no_domain_redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "ldap-scope.internal",
        reverse_tag,
    )

    _replace_server_config(server_host, 'group_ids = ["admins"]', 'group_ids = ["engineering", "admins", "support"]')
    xp2p_server_runner("server", "identity", "sync", check=True)
    restored = _identity_state(server_host)["current"]
    assert "engineering" in restored["groups"]
    assert "usr-10003" in restored["subjects"]


@pytest.mark.host
@pytest.mark.linux
def test_identity_keycloak_authoritative_user_and_group_removal(
    server_host, aux_host, xp2p_server_runner, keycloak_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62061", SERVER_HOST)
    token = _keycloak_token(server_host)
    _write_server_config(server_host, _keycloak_provider_config(token))
    xp2p_server_runner("server", "identity", "sync", check=True)
    first = _identity_state(server_host)["current"]
    alice_label = first["subjects"]["11111111-1111-4111-8111-111111111111"]["user_label"]
    carol_label = first["subjects"]["33333333-3333-4333-8333-333333333333"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)
    _provision_identity(xp2p_server_runner, carol_label)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "keycloak-owned-removal.internal", reverse_tag, "engineering")
    _identity_keycloak.run_keycloak(aux_host, "remove-user", "alice")
    xp2p_server_runner("server", "identity", "sync", check=True)
    assert "11111111-1111-4111-8111-111111111111" not in _identity_state(server_host)["current"]["subjects"]
    assert alice_label not in _server_user_labels(server_host, xp2p_server_runner)
    assert reverse_tag not in _server_reverse(server_host)

    carol_tag = helpers.expected_reverse_tag(carol_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "keycloak-group-removal.internal", carol_tag, "sales")
    desired_before = helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE)
    _identity_keycloak.run_keycloak(aux_host, "remove-group", "sales")
    xp2p_server_runner("server", "identity", "sync", check=True)
    assert "sales" not in _identity_state(server_host)["current"]["groups"]
    assert helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE) == desired_before
    helpers.assert_no_domain_redirect_rule(
        helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True),
        "keycloak-group-removal.internal",
        carol_tag,
    )


@pytest.mark.host
@pytest.mark.linux
def test_identity_stale_cache_recovers_after_sync_and_different_provider_detaches(
    server_host, xp2p_server_runner, ldap_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62062", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config(max_cache_age="24h"))
    xp2p_server_runner("server", "identity", "sync", check=True)
    label = _identity_state(server_host)["current"]["subjects"]["usr-10001"]["user_label"]
    _provision_identity(xp2p_server_runner, label)
    reverse_tag = helpers.expected_reverse_tag(label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-stale-recovery.internal", reverse_tag, "admins")

    _replace_server_config(server_host, 'max_cache_age = "24h"', 'max_cache_age = "1ns"')
    time.sleep(0.01)
    stale = xp2p_server_runner("server", "render", "xray", "--desired", check=False)
    assert stale.rc != 0
    assert "identity cache is stale" in _combined(stale)

    _replace_server_config(server_host, 'max_cache_age = "1ns"', 'max_cache_age = "24h"')
    xp2p_server_runner("server", "identity", "sync", check=True)
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    assert label in (_redirect_rule(routing, "ldap-stale-recovery.internal", reverse_tag).get("user") or [])

    xp2p_server_runner("server", "identity", "select", "replacement", "--kind", "scim", "--group", "admins", check=True)
    state = _identity_state(server_host)
    assert state["provider"]["instance_id"] == "replacement"
    assert state["current"]["detached"] is True
    detached_render = xp2p_server_runner("server", "render", "xray", "--desired", check=False)
    assert detached_render.rc != 0
    assert "identity cache is not available" in _combined(detached_render)


def _add_explicit_user_redirect(xp2p_server_runner, domain: str, tag: str, *users: str) -> None:
    args = ["server", "redirect", "add", "--domain", domain, "--tag", tag, "--access", "restricted"]
    for user in users:
        args.extend(["--allow-user", user])
    xp2p_server_runner(*args, check=True)


def _server_user_labels(server_host, xp2p_server_runner) -> set[str]:
    return {client["email"] for client in user_helpers.trojan_clients(server_host, xp2p_server_runner)}


def _server_reverse(server_host) -> dict:
    return (helpers.read_toml(server_host, helpers.SERVER_CONFIG_FILE).get("server") or {}).get("reverse_channels") or {}


def _desired_redirect_users(server_host, domain: str) -> list[str]:
    redirects = (helpers.read_toml(server_host, helpers.SERVER_CONFIG_FILE).get("server") or {}).get("server_redirects") or []
    for rule in redirects:
        if rule.get("domain") == domain:
            return rule.get("users") or rule.get("allow_users") or []
    raise AssertionError(f"redirect {domain} not found in Desired")
