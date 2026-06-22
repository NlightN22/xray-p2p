from __future__ import annotations

from . import _identity_keycloak


def test_identity_keycloak_provider(aux_host, server_host, keycloak_directory):
    users = _identity_keycloak.query_keycloak(server_host, "list-users")
    by_name = {user["username"]: user for user in users}
    assert by_name["alice"]["id"] == "11111111-1111-4111-8111-111111111111"
    assert by_name["dave"]["email"] == "dave@example.test"

    groups = _identity_keycloak.query_keycloak(server_host, "list-groups")
    assert {group["name"] for group in groups} >= {"engineering", "admins", "sales", "empty"}

    _identity_keycloak.run_keycloak(aux_host, "set-membership", "dave", "engineering", "present")
    members = _identity_keycloak.query_keycloak(server_host, "list-group-members", "engineering")
    assert any(member["id"] == "44444444-4444-4444-8444-444444444444" for member in members)
