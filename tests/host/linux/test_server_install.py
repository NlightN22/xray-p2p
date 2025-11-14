from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers

SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"
SERVER_OUTBOUNDS = helpers.SERVER_CONFIG_DIR / "outbounds.json"
SERVER_LOGS_JSON = helpers.SERVER_CONFIG_DIR / "logs.json"
SERVER_ROUTING_JSON = helpers.SERVER_CONFIG_DIR / "routing.json"
SERVER_CERT_DEST = helpers.SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = helpers.SERVER_CONFIG_DIR / "key.pem"


def _cleanup(server_host, xp2p_server_runner) -> None:
    helpers.cleanup_server_install(server_host, xp2p_server_runner)


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Trojan inbound not found in configuration")


@pytest.mark.host
@pytest.mark.linux
def test_server_install_uses_provided_certificate_and_force_overwrites(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    cert_source = PurePosixPath("/tmp/xp2p-server-cert.pem")
    key_source = PurePosixPath("/tmp/xp2p-server-key.pem")
    first_cert = "CERTIFICATE-DATA-ONE"
    second_cert = "CERTIFICATE-DATA-TWO"
    first_key = "KEY-DATA-ONE"
    second_key = "KEY-DATA-TWO"
    helpers.write_text(server_host, cert_source, first_cert)
    helpers.write_text(server_host, key_source, first_key)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62001",
            "--host",
            "xp2p.test.local",
            "--cert",
            cert_source.as_posix(),
            "--key",
            key_source.as_posix(),
            "--force",
            check=True,
        )

        assert helpers.path_exists(server_host, helpers.XRAY_BINARY), f"Expected xray binary at {helpers.XRAY_BINARY}"
        for config_path in (
            SERVER_INBOUNDS,
            SERVER_OUTBOUNDS,
            SERVER_LOGS_JSON,
            SERVER_ROUTING_JSON,
        ):
            assert helpers.path_exists(server_host, config_path), f"Missing config file {config_path}"

        helpers.write_text(server_host, cert_source, second_cert)
        helpers.write_text(server_host, key_source, second_key)
        xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cert",
            cert_source.as_posix(),
            "--key",
            key_source.as_posix(),
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds)
        assert trojan.get("port") == 62001
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == SERVER_CERT_DEST.as_posix()
        assert primary_cert.get("keyFile") == SERVER_KEY_DEST.as_posix()
        assert helpers.read_text(server_host, SERVER_CERT_DEST).strip() == second_cert
        assert helpers.read_text(server_host, SERVER_KEY_DEST).strip() == second_key
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_install_requires_force_when_state_exists(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62011",
            "--host",
            "state-required.example",
            "--force",
            check=True,
        )

        result = xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62012",
            "--host",
            "state-required-2.example",
            check=False,
        )
        assert result.rc != 0, "Expected server install to fail without --force when state exists"
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "server files already present" in combined
    finally:
        _cleanup(server_host, xp2p_server_runner)
