from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _identity_keycloak
from tests.host.linux import _identity_ldap
from tests.host.linux import _server_user_helpers as user_helpers
from tests.host.linux import env as linux_env
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
def test_identity_ldap_new_group_member_requires_provisioning(server_host, aux_host, xp2p_server_runner, ldap_directory):
    user_helpers.install_server(server_host, xp2p_server_runner, "62053", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)
    initial = _identity_state(server_host)
    initial_engineering = set(initial["current"]["groups"]["engineering"]["direct_members"])

    _identity_ldap.run_ldap(aux_host, "add-user", "usr-10901", "Erin Example", "erin", "erin@example.test")
    _identity_ldap.run_ldap(aux_host, "set-membership", "erin", "engineering", "present")
    xp2p_server_runner("server", "identity", "sync", check=True)

    state = _identity_state(server_host)
    current = state["current"]
    assert "usr-10901" in current["subjects"]
    erin_label = current["subjects"]["usr-10901"]["user_label"]
    assert erin_label.startswith("idp-")
    assert current["subjects"]["usr-10901"].get("provisioned") in (None, False)
    assert set(current["groups"]["engineering"]["direct_members"]) == initial_engineering | {"usr-10901"}

    alice_label = current["subjects"]["usr-10001"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)
    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-new-member.internal", reverse_tag, "engineering")
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    assert erin_label not in (_redirect_rule(routing, "ldap-new-member.internal", reverse_tag).get("user") or [])

    _provision_identity(xp2p_server_runner, erin_label)
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    assert erin_label in (_redirect_rule(routing, "ldap-new-member.internal", reverse_tag).get("user") or [])


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_group_removal_updates_acl_without_unprovisioning(server_host, aux_host, xp2p_server_runner, ldap_directory):
    user_helpers.install_server(server_host, xp2p_server_runner, "62054", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)
    label = _identity_state(server_host)["current"]["subjects"]["usr-10001"]["user_label"]
    _provision_identity(xp2p_server_runner, label)
    reverse_tag = helpers.expected_reverse_tag(label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-membership.internal", reverse_tag, "engineering")
    assert label in (_redirect_rule(helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True), "ldap-membership.internal", reverse_tag).get("user") or [])

    _identity_ldap.run_ldap(aux_host, "set-membership", "alice", "engineering", "absent")
    xp2p_server_runner("server", "identity", "sync", check=True)

    state = _identity_state(server_host)
    assert "usr-10001" in state["current"]["subjects"]
    assert state["current"]["subjects"]["usr-10001"]["provisioned"] is True
    assert "usr-10001" not in state["current"]["groups"]["engineering"].get("direct_members", [])
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    helpers.assert_no_domain_redirect_rule(routing, "ldap-membership.internal", reverse_tag)


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_deleted_and_empty_groups_fail_closed_without_rewriting_desired(
    server_host, xp2p_server_runner, aux_host, ldap_directory
):
    user_helpers.install_server(server_host, xp2p_server_runner, "62055", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)
    label = _identity_state(server_host)["current"]["subjects"]["usr-10001"]["user_label"]
    _provision_identity(xp2p_server_runner, label)
    reverse_tag = helpers.expected_reverse_tag(label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "ldap-empty.internal", reverse_tag, "empty")
    _add_group_redirect(xp2p_server_runner, "ldap-deleted-group.internal", reverse_tag, "engineering")
    desired_before = helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE)

    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    helpers.assert_no_domain_redirect_rule(routing, "ldap-empty.internal", reverse_tag)

    _identity_ldap.run_ldap(aux_host, "remove-group", "engineering")
    xp2p_server_runner("server", "identity", "sync", check=True)

    state = _identity_state(server_host)
    assert "engineering" not in state["current"]["groups"]
    assert helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE) == desired_before
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    helpers.assert_no_domain_redirect_rule(routing, "ldap-deleted-group.internal", reverse_tag)


@pytest.mark.host
@pytest.mark.linux
def test_identity_keycloak_membership_changes_and_repeated_sync_keep_labels(
    server_host, aux_host, xp2p_server_runner, keycloak_directory
):
    token = _keycloak_token(server_host)
    _write_server_config(server_host, _keycloak_provider_config(token))
    xp2p_server_runner("server", "identity", "sync", check=True)
    first = _identity_state(server_host)
    alice_label = first["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["user_label"]
    _provision_identity(xp2p_server_runner, alice_label)

    _identity_keycloak.run_keycloak(
        aux_host,
        "add-user",
        "99999999-9999-4999-8999-999999999999",
        "erin",
        "Erin",
        "Example",
        "erin@example.test",
    )
    _identity_keycloak.run_keycloak(aux_host, "set-membership", "erin", "engineering", "present")
    xp2p_server_runner("server", "identity", "sync", check=True)
    second = _identity_state(server_host)
    new_subjects = set(second["current"]["subjects"]) - set(first["current"]["subjects"])
    assert len(new_subjects) == 1
    erin = second["current"]["subjects"][new_subjects.pop()]
    assert erin["user_label"].startswith("idp-")
    assert erin.get("provisioned") in (None, False)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    _add_group_redirect(xp2p_server_runner, "keycloak-membership.internal", reverse_tag, "engineering")
    rule = _redirect_rule(helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True), "keycloak-membership.internal", reverse_tag)
    assert erin["user_label"] not in (rule.get("user") or [])

    _identity_keycloak.run_keycloak(aux_host, "set-membership", "alice", "engineering", "absent")
    xp2p_server_runner("server", "identity", "sync", check=True)
    third = _identity_state(server_host)
    assert third["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["user_label"] == alice_label
    assert third["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["provisioned"] is True
    assert alice_label not in _labels_for_group(third["current"], "engineering")

    xp2p_server_runner("server", "identity", "sync", check=True)
    repeated = _identity_state(server_host)
    assert repeated["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["user_label"] == alice_label
    assert repeated["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["provisioned"] is True


@pytest.mark.host
@pytest.mark.linux
def test_identity_failed_sync_keeps_current_generation_and_hides_secrets(server_host, xp2p_server_runner, ldap_directory):
    _write_server_config(server_host, _ldap_provider_config())
    xp2p_server_runner("server", "identity", "sync", check=True)
    before = _identity_state(server_host)["current"]

    _replace_server_config(server_host, 'secret = "integration-reader-password"', 'secret = "wrong-password"')
    result = xp2p_server_runner("server", "identity", "sync", check=False)
    assert result.rc != 0
    assert "wrong-password" not in _combined(result)
    state = _identity_state(server_host)
    assert state["status"]["state"] == "error"
    assert state["current"] == before
    status = xp2p_server_runner("server", "identity", "status", check=True).stdout or ""
    assert "wrong-password" not in status


def _provision_identity(xp2p_server_runner, label: str) -> None:
    xp2p_server_runner("server", "identity", "provision", label, "--host", SERVER_HOST, check=True)


def _add_group_redirect(xp2p_server_runner, domain: str, tag: str, group: str) -> None:
    xp2p_server_runner(
        "server",
        "redirect",
        "add",
        "--domain",
        domain,
        "--tag",
        tag,
        "--access",
        "restricted",
        "--allow-group",
        group,
        check=True,
    )


def _keycloak_token(server_host) -> str:
    return linux_env.run_guest_script_with_env(
        server_host,
        "scripts/linux/identity_keycloak.sh",
        {"KEYCLOAK_URL": f"http://{_identity_keycloak.KEYCLOAK_HOST}:8080"},
        "token",
    ).stdout.strip()
