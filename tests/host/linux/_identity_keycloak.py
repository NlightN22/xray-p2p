from __future__ import annotations

import json
from contextlib import contextmanager

from testinfra.host import Host

from . import env as linux_env

KEYCLOAK_HOST = "10.62.10.13"


def run_keycloak(host: Host, *args: str):
    result = linux_env.run_guest_script(host, "scripts/linux/identity_keycloak.sh", *args)
    if result.rc != 0:
        raise RuntimeError(f"Keycloak command failed: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return result


def query_keycloak(host: Host, action: str, *args: str):
    result = linux_env.run_guest_script_with_env(
        host, "scripts/linux/identity_keycloak.sh", {"KEYCLOAK_URL": f"http://{KEYCLOAK_HOST}:8080"}, action, *args
    )
    if result.rc != 0:
        raise RuntimeError(f"Keycloak query failed:\n{result.stdout}\n{result.stderr}")
    return json.loads(result.stdout)


@contextmanager
def keycloak_directory(aux_host: Host):
    run_keycloak(aux_host, "prepare")
    try:
        yield
    finally:
        run_keycloak(aux_host, "cleanup")
