from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _identity_keycloak
from tests.host.linux import _identity_ldap
from tests.host.linux import _server_user_helpers as user_helpers
from tests.host.linux import env as linux_env

IDENTITY_STATE = helpers.STATE_ROOT / "identity" / "state.json"
SERVER_HOST = "10.62.10.11"


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_sync_provisioning_acl_and_detach(server_host, xp2p_server_runner, ldap_directory):
    user_helpers.install_server(server_host, xp2p_server_runner, "62051", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config(max_cache_age="24h"))

    xp2p_server_runner("server", "identity", "sync", check=True)
    state = _identity_state(server_host)
    assert state["status"]["state"] == "success"
    current = state["current"]
    engineering_labels = _labels_for_group(current, "engineering")
    assert len(engineering_labels) == 2
    assert current["subjects"]["usr-10001"]["user_label"] == "idp-s4jyyvcybikicve55n2yaculim@xp2p.local"

    alice_label = current["subjects"]["usr-10001"]["user_label"]
    carol_label = current["subjects"]["usr-10003"]["user_label"]
    provision = xp2p_server_runner(
        "server",
        "identity",
        "provision",
        alice_label,
        "--host",
        SERVER_HOST,
        check=True,
    )
    assert "trojan://" in (provision.stdout or "")
    state = _identity_state(server_host)
    assert state["current"]["subjects"]["usr-10001"]["provisioned"] is True
    xp2p_server_runner(
        "server",
        "identity",
        "provision",
        carol_label,
        "--host",
        SERVER_HOST,
        check=True,
    )
    state = _identity_state(server_host)
    assert state["current"]["subjects"]["usr-10003"]["provisioned"] is True
    clients = user_helpers.trojan_clients(server_host, xp2p_server_runner)
    assert any(item["email"] == alice_label for item in clients)
    assert any(item["email"] == carol_label for item in clients)

    reverse_tag = helpers.expected_reverse_tag(alice_label, SERVER_HOST)
    xp2p_server_runner(
        "server",
        "redirect",
        "add",
        "--domain",
        "engineering.internal",
        "--tag",
        reverse_tag,
        "--access",
        "restricted",
        "--allow-group",
        "engineering",
        check=True,
    )
    routing = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
    rule = _redirect_rule(routing, "engineering.internal", reverse_tag)
    assert sorted(rule.get("user") or []) == sorted(engineering_labels)

    status = xp2p_server_runner("server", "identity", "status", check=True).stdout or ""
    assert "integration-reader-password" not in status
    assert "user " in status and "group engineering" in status

    xp2p_server_runner("server", "identity", "detach", check=True)
    detached = _identity_state(server_host)
    assert detached["status"]["state"] == "detached"
    assert detached["current"]["detached"] is True

    stale_render = xp2p_server_runner("server", "render", "xray", "--desired", check=False)
    assert stale_render.rc != 0
    assert "identity cache is not available" in _combined(stale_render)

    xp2p_server_runner("server", "identity", "select", "ldap-e2e", "--kind", "ldap", "--group", "engineering", check=True)
    reattached = _identity_state(server_host)
    assert reattached["current"].get("detached") in (None, False)


@pytest.mark.host
@pytest.mark.linux
def test_identity_ldap_sync_reports_bind_failure(server_host, xp2p_server_runner, ldap_directory):
    _write_server_config(server_host, _ldap_provider_config(secret="wrong-password"))

    result = xp2p_server_runner("server", "identity", "sync", check=False)
    assert result.rc != 0
    state = _identity_state(server_host)
    assert state["status"]["state"] == "error"
    assert state.get("current") is None
    assert "wrong-password" not in _combined(result)


@pytest.mark.host
@pytest.mark.linux
def test_identity_keycloak_scim_sync_and_membership_change(server_host, aux_host, xp2p_server_runner, keycloak_directory):
    token = linux_env.run_guest_script_with_env(
        server_host,
        "scripts/linux/identity_keycloak.sh",
        {"KEYCLOAK_URL": f"http://{_identity_keycloak.KEYCLOAK_HOST}:8080"},
        "token",
    ).stdout.strip()
    _write_server_config(server_host, _keycloak_provider_config(token))

    xp2p_server_runner("server", "identity", "sync", check=True)
    first = _identity_state(server_host)
    first_engineering = _labels_for_group(first["current"], "engineering")
    assert len(first_engineering) == 2
    assert (
        first["current"]["subjects"]["11111111-1111-4111-8111-111111111111"]["user_label"]
        == "idp-7puc4dzzjx5ludkzrknccvaxlg@xp2p.local"
    )

    _identity_keycloak.run_keycloak(aux_host, "set-membership", "dave", "engineering", "present")
    xp2p_server_runner("server", "identity", "sync", check=True)
    second = _identity_state(server_host)
    second_engineering = _labels_for_group(second["current"], "engineering")
    assert len(second_engineering) == 3
    assert set(first_engineering).issubset(set(second_engineering))
    assert second["status"]["state"] == "success"
    assert token not in (xp2p_server_runner("server", "identity", "status", check=True).stdout or "")


@pytest.mark.host
@pytest.mark.linux
def test_identity_cache_staleness_blocks_identity_dependent_compile(server_host, xp2p_server_runner, ldap_directory):
    user_helpers.install_server(server_host, xp2p_server_runner, "62052", SERVER_HOST)
    _append_server_config(server_host, _ldap_provider_config(max_cache_age="24h"))
    xp2p_server_runner("server", "identity", "sync", check=True)
    state = _identity_state(server_host)
    label = state["current"]["subjects"]["usr-10001"]["user_label"]
    xp2p_server_runner("server", "identity", "provision", label, "--host", SERVER_HOST, check=True)
    reverse_tag = helpers.expected_reverse_tag(label, SERVER_HOST)
    xp2p_server_runner(
        "server",
        "redirect",
        "add",
        "--domain",
        "stale.internal",
        "--tag",
        reverse_tag,
        "--access",
        "restricted",
        "--allow-group",
        "admins",
        check=True,
    )

    _replace_server_config(server_host, 'max_cache_age = "24h"', 'max_cache_age = "1ns"')
    time.sleep(0.01)
    result = xp2p_server_runner("server", "render", "xray", "--desired", check=False)
    assert result.rc != 0
    assert "identity cache is stale" in _combined(result)


def _write_server_config(host, content: str) -> None:
    helpers.write_text(host, helpers.SERVER_CONFIG_FILE, content)


def _append_server_config(host, content: str) -> None:
    existing = helpers.read_text(host, helpers.SERVER_CONFIG_FILE)
    helpers.write_text(host, helpers.SERVER_CONFIG_FILE, existing + "\n" + content)


def _replace_server_config(host, old: str, new: str) -> None:
    existing = helpers.read_text(host, helpers.SERVER_CONFIG_FILE)
    assert old in existing
    helpers.write_text(host, helpers.SERVER_CONFIG_FILE, existing.replace(old, new, 1))


def _ldap_provider_config(*, secret: str = "integration-reader-password", max_cache_age: str = "24h") -> str:
    return f"""
[server.identity_provider]
instance_id = "ldap-e2e"
kind = "ldap"
secret = "{secret}"
group_ids = ["engineering", "admins"]
max_cache_age = "{max_cache_age}"

[server.identity_provider.ldap]
url = "ldap://{_identity_ldap.LDAP_HOST}:389"
base_dn = "{_identity_ldap.BASE_DN}"
bind_dn = "cn=ldap-reader,ou=service,{_identity_ldap.BASE_DN}"
user_filter = "(objectClass=inetOrgPerson)"
group_filter = "(objectClass=posixGroup)"
subject_attribute = "employeeNumber"
membership_attribute = "memberUid"
display_name_attribute = "displayName"
"""


def _keycloak_provider_config(token: str) -> str:
    return f"""
[server.identity_provider]
instance_id = "keycloak-e2e"
kind = "scim"
secret = "{token}"
group_ids = ["engineering", "admins"]

[server.identity_provider.scim]
endpoint = "http://{_identity_keycloak.KEYCLOAK_HOST}:8080/admin/realms/xp2p-identity-test"
timeout = "20s"
"""


def _identity_state(host) -> dict:
    return linux_env.read_json(host, IDENTITY_STATE)


def _labels_for_group(current: dict, group_id: str) -> list[str]:
    group = current["groups"][group_id]
    subjects = current["subjects"]
    return [subjects[subject_id]["user_label"] for subject_id in group.get("direct_members", [])]


def _redirect_rule(routing: dict, domain: str, tag: str) -> dict:
    expected = f"domain:{domain}"
    for rule in routing.get("routing", {}).get("rules", []):
        if rule.get("outboundTag") != tag:
            continue
        if expected in (rule.get("domains") or []):
            return rule
    raise AssertionError(f"Redirect rule for {domain} via {tag} not found")


def _combined(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()
