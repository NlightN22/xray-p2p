import json

import pytest

CLIENT_CONFIG_OUTBOUNDS = r"C:\Program Files\xp2p\config-client\outbounds.json"
CLIENT_LOG_FILE = r"C:\Program Files\xp2p\logs\client.err"


def _cleanup_client_install(runner) -> None:
    runner("client", "remove", "--ignore-missing")


def _read_remote_json(client_host, path: str) -> dict:
    file_obj = client_host.file(path)
    assert file_obj.exists, f"Expected file {path} to exist on client guest"
    content = file_obj.content_string
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


@pytest.mark.host
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.10",
            "--password",
            "test_password123",
            "--force",
            check=True,
        )

        data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        primary = data["outbounds"][0]["settings"]["servers"][0]
        assert primary["address"] == "10.0.10.10"
        assert primary["password"] == "test_password123"
        assert data["outbounds"][0]["streamSettings"]["tlsSettings"]["serverName"] == "10.0.10.10"

        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.11",
            "--password",
            "override_password456",
            "--server-name",
            "vpn.example.local",
            "--force",
            check=True,
        )

        updated_data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        primary_updated = updated_data["outbounds"][0]["settings"]["servers"][0]
        assert primary_updated["address"] == "10.0.10.11"
        assert primary_updated["password"] == "override_password456"
        assert (
            updated_data["outbounds"][0]["streamSettings"]["tlsSettings"]["serverName"]
            == "vpn.example.local"
        )
    finally:
        _cleanup_client_install(xp2p_client_runner)


@pytest.mark.host
def test_client_run_starts_xray_core(client_host, xp2p_client_runner, xp2p_client_run_factory):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.10",
            "--password",
            "runtime_password789",
            "--force",
            check=True,
        )

        with xp2p_client_run_factory() as session:
            assert session["pid"] > 0

        log_file = client_host.file(CLIENT_LOG_FILE)
        assert log_file.exists, f"Expected log file {CLIENT_LOG_FILE} to be created"
        log_content = log_file.content_string
        assert "Xray 25" in log_content or "core: Xray" in log_content
        assert "Failed to start" not in log_content
    finally:
        _cleanup_client_install(xp2p_client_runner)
