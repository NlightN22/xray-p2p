import json
from contextlib import suppress
from uuid import uuid4

import pytest

from .helpers import (
    ensure_stage_one,
    get_inbound_client_emails,
    load_clients_registry,
    load_inbounds_config,
    server_user_issue,
    server_user_remove,
)


@pytest.fixture(scope="module")
def server_connection_host():
    return "10.0.0.1"


def _make_unique_email() -> str:
    return f"pytest-{uuid4().hex}@auto.local"


@pytest.fixture(scope="module", autouse=True)
def ensure_stage_one_prereq(host_r2):
    ensure_stage_one(host_r2, "r2-client", "10.0.102.0/24")


def test_server_user_issue_registers_client(host_r1, server_connection_host):
    email = _make_unique_email()
    clients_before = load_clients_registry(host_r1)
    inbounds_before = load_inbounds_config(host_r1)

    issued = False
    try:
        issue_result = server_user_issue(host_r1, email, server_connection_host)
        issued = True
        combined_issue_output = f"{issue_result.stdout}\n{issue_result.stderr}".lower()
        assert "trojan://" in combined_issue_output, "Trojan link not returned in output"

        clients_after = load_clients_registry(host_r1)
        assert any(c.get("email") == email for c in clients_after), "Client not added"
        assert len(clients_after) == len(clients_before) + 1, "Client count did not increase"

        inbounds_after = load_inbounds_config(host_r1)
        assert email in get_inbound_client_emails(inbounds_after), "Inbound entry missing"
    finally:
        if issued:
            with suppress(AssertionError):
                server_user_remove(host_r1, email)

    clients_final = load_clients_registry(host_r1)
    assert len(clients_final) == len(clients_before)
    assert email not in {c.get("email") for c in clients_final}

    inbounds_final = load_inbounds_config(host_r1)
    assert get_inbound_client_emails(inbounds_final).count(email) == 0
    assert json.dumps(inbounds_final, sort_keys=True) == json.dumps(
        inbounds_before, sort_keys=True
    )


def test_server_user_remove_purges_client(host_r1, server_connection_host):
    email = _make_unique_email()
    issued = False
    try:
        server_user_issue(host_r1, email, server_connection_host)
        issued = True

        clients_mid = load_clients_registry(host_r1)
        assert any(c.get("email") == email for c in clients_mid), "Client missing before removal"

        remove_result = server_user_remove(host_r1, email)
        combined_remove_output = f"{remove_result.stdout}\n{remove_result.stderr}".lower()
        assert "removed" in combined_remove_output, "Removal confirmation missing"

        clients_after = load_clients_registry(host_r1)
        assert email not in {c.get("email") for c in clients_after}

        inbounds_after = load_inbounds_config(host_r1)
        assert email not in get_inbound_client_emails(inbounds_after)
    finally:
        if issued:
            with suppress(AssertionError):
                server_user_remove(host_r1, email)
