import json
from uuid import uuid4

import pytest

from .helpers import (
    SERVER_CONFIG_DIR,
    ensure_stage_one,
    get_inbound_client_emails,
    load_clients_registry,
    load_inbounds_config,
    server_install,
    server_is_installed,
    server_remove,
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


@pytest.mark.serial
def test_server_user_full_flow(host_r1, server_connection_host):
    email = _make_unique_email()

    server_remove(host_r1, purge_core=True, check=False)
    assert not server_is_installed(host_r1), "Server must be absent before provisioning"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Server config directory should be removed"
    assert load_clients_registry(host_r1) == [], "Client registry should be empty after cleanup"
    assert get_inbound_client_emails(load_inbounds_config(host_r1)) == [], "Inbound clients should not linger"

    install_env = {
        "XRAY_REISSUE_CERT": "1",
        "XRAY_SKIP_PORT_CHECK": "1",
    }
    try:
        server_install(host_r1, "pytest-user", "9666", "--force", env=install_env)
        assert server_is_installed(host_r1), "Server should report as installed after provisioning"

        clients_before = load_clients_registry(host_r1)
        inbounds_before = load_inbounds_config(host_r1)

        issue_result = server_user_issue(host_r1, email, server_connection_host)
        combined_issue_output = f"{issue_result.stdout}\n{issue_result.stderr}".lower()
        assert "trojan://" in combined_issue_output, "Trojan link not returned in output"

        clients_after = load_clients_registry(host_r1)
        assert any(c.get("email") == email for c in clients_after), "Client not added to registry"
        assert len(clients_after) == len(clients_before) + 1, "Client count did not increase"

        inbounds_after = load_inbounds_config(host_r1)
        assert email in get_inbound_client_emails(inbounds_after), "Inbound entry missing for issued client"

        remove_result = server_user_remove(host_r1, email)
        combined_remove_output = f"{remove_result.stdout}\n{remove_result.stderr}".lower()
        assert "removed" in combined_remove_output, "Removal confirmation missing"

        clients_final = load_clients_registry(host_r1)
        assert json.dumps(clients_final, sort_keys=True) == json.dumps(
            clients_before, sort_keys=True
        ), "Client registry did not revert to original state"

        inbounds_final = load_inbounds_config(host_r1)
        assert json.dumps(inbounds_final, sort_keys=True) == json.dumps(
            inbounds_before, sort_keys=True
        ), "Inbound config did not revert to original state"
    finally:
        server_remove(host_r1, purge_core=True, check=False)

    assert not server_is_installed(host_r1), "Server should be removed after cleanup"
    assert not host_r1.file(SERVER_CONFIG_DIR).exists, "Cleanup should remove server config directory"
    assert load_clients_registry(host_r1) == [], "Client registry should be empty after cleanup"
    assert get_inbound_client_emails(load_inbounds_config(host_r1)) == [], "Inbound clients should not linger after cleanup"
