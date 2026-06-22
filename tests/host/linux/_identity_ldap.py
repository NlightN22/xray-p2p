from __future__ import annotations

from contextlib import contextmanager

from testinfra.host import Host

from . import env as linux_env

LDAP_HOST = "10.62.10.13"
BASE_DN = "dc=identity,dc=xp2p,dc=test"


def run_ldap(host: Host, *args: str):
    result = linux_env.run_guest_script(host, "scripts/linux/identity_ldap.sh", *args)
    if result.rc != 0:
        raise RuntimeError(f"LDAP command failed: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return result


def query_ldap(host: Host, ldap_filter: str, *attributes: str):
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/identity_ldap_query.sh",
        LDAP_HOST,
        BASE_DN,
        ldap_filter,
        *attributes,
    )
    if result.rc != 0:
        raise RuntimeError(f"LDAP query failed:\n{result.stdout}\n{result.stderr}")
    return result.stdout


@contextmanager
def ldap_directory(aux_host: Host):
    run_ldap(aux_host, "prepare")
    try:
        yield
    finally:
        run_ldap(aux_host, "cleanup")
