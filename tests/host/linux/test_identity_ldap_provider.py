from __future__ import annotations

from . import _identity_ldap


def test_identity_ldap_provider(aux_host, server_host, ldap_directory):
    users = _identity_ldap.query_ldap(server_host, "(objectClass=inetOrgPerson)", "uid", "employeeNumber", "mail")
    assert "uid: alice" in users
    assert "employeeNumber: usr-10001" in users
    assert "uid: dave" in users

    groups = _identity_ldap.query_ldap(server_host, "(objectClass=posixGroup)", "cn", "gidNumber", "memberUid")
    assert "cn: engineering" in groups
    assert "gidNumber: 20001" in groups
    assert "cn: empty" in groups

    _identity_ldap.run_ldap(aux_host, "set-membership", "dave", "engineering", "present")
    engineering = _identity_ldap.query_ldap(server_host, "(cn=engineering)", "memberUid")
    assert "memberUid: usr-10004" in engineering
