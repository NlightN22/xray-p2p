from __future__ import annotations

import json
from pathlib import Path, PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers

SERVER_EXT_DIR = helpers.CONFIG_ROOT / helpers.SERVER_CONFIG_DIR_NAME
SERVER_CERT_DEST = helpers.CONFIG_ROOT / "tls" / "server" / "cert.pem"
SERVER_KEY_DEST = helpers.CONFIG_ROOT / "tls" / "server" / "key.pem"
FIXTURE_CERT = Path("tests/fixtures/tls/integration-cert.pem")
FIXTURE_KEY = Path("tests/fixtures/tls/integration-key.pem")


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Proxy inbound not found in configuration")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".strip()


def _read_cert_state(runner) -> dict:
    result = runner(
        "--json",
        "server",
        "cert",
        "state",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        check=False,
    )
    assert result.rc == 0, (
        f"Expected cert state to succeed, rc={result.rc}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    return json.loads(result.stdout or "{}").get("result", {})


def _parse_self_signed(state: dict) -> bool:
    value = state.get("self_signed")
    assert isinstance(value, bool), f"Invalid cert state self_signed value: {state}"
    return value


@pytest.mark.host
@pytest.mark.linux
def test_server_install_uses_provided_certificate_and_force_overwrites(server_host, xp2p_server_runner):
    helpers.remove_path(server_host, SERVER_CERT_DEST)
    helpers.remove_path(server_host, SERVER_KEY_DEST)
    cert_source = PurePosixPath("/tmp/xp2p-server-cert.pem")
    key_source = PurePosixPath("/tmp/xp2p-server-key.pem")
    cert_content = FIXTURE_CERT.read_text(encoding="utf-8")
    key_content = FIXTURE_KEY.read_text(encoding="utf-8")
    helpers.write_text(server_host, cert_source, cert_content)
    helpers.write_text(server_host, key_source, key_content)
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
            helpers.SERVER_CONFIG_FILE,
            SERVER_EXT_DIR / "routing.rules.after-xp2p-system.json",
            SERVER_EXT_DIR / "routing.rules.after-xp2p-managed.json",
            SERVER_EXT_DIR / "inbounds.append.json",
            SERVER_EXT_DIR / "outbounds.append.json",
        ):
            assert helpers.path_exists(server_host, config_path), f"Missing desired input {config_path}"

        helpers.write_text(server_host, cert_source, cert_content)
        helpers.write_text(server_host, key_source, key_content)
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

        inbounds = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
        trojan = _trojan_inbound(inbounds)
        assert trojan.get("port") == 62001
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        expected_cert = SERVER_CERT_DEST.as_posix()
        expected_key = SERVER_KEY_DEST.as_posix()
        assert primary_cert.get("certificateFile") == expected_cert
        assert primary_cert.get("keyFile") == expected_key
        assert _parse_self_signed(_read_cert_state(xp2p_server_runner)) in {True, False}
        assert "allowInsecure" not in tls_settings
        assert helpers.path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to exist in config-server"
        assert helpers.path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to exist in config-server"
    finally:
        helpers.remove_path(server_host, cert_source)


@pytest.mark.host
@pytest.mark.linux
def test_server_install_generates_self_signed_certificate(server_host, xp2p_server_runner):
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62015",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        assert helpers.path_exists(server_host, SERVER_CERT_DEST), f"Expected certificate at {SERVER_CERT_DEST}"
        assert helpers.path_exists(server_host, SERVER_KEY_DEST), f"Expected key at {SERVER_KEY_DEST}"

        inbounds = helpers.render_xray(server_host, xp2p_server_runner, "server", desired=True)
        trojan = _trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        expected_cert = SERVER_CERT_DEST.as_posix()
        expected_key = SERVER_KEY_DEST.as_posix()
        assert primary_cert.get("certificateFile") == expected_cert
        assert primary_cert.get("keyFile") == expected_key

        state = _read_cert_state(xp2p_server_runner)
        assert state.get("status") == "ok"
        assert state.get("self_signed") is True
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_mismatched_cert_key(server_host, xp2p_server_runner):
    cert_source = PurePosixPath("/tmp/xp2p-mismatch-cert.pem")
    cert_content = FIXTURE_CERT.read_text(encoding="utf-8")
    existing_key = SERVER_KEY_DEST
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62005",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        assert helpers.path_exists(server_host, existing_key), f"Expected generated key at {existing_key}"
        before_cert = helpers.file_sha256(server_host, SERVER_CERT_DEST) if helpers.path_exists(server_host, SERVER_CERT_DEST) else ""
        before_key = helpers.file_sha256(server_host, SERVER_KEY_DEST) if helpers.path_exists(server_host, SERVER_KEY_DEST) else ""
        helpers.write_text(server_host, cert_source, cert_content)

        result = xp2p_server_runner(
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
            existing_key.as_posix(),
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected mismatched certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate and key do not match" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
        after_cert = helpers.file_sha256(server_host, SERVER_CERT_DEST) if helpers.path_exists(server_host, SERVER_CERT_DEST) else ""
        after_key = helpers.file_sha256(server_host, SERVER_KEY_DEST) if helpers.path_exists(server_host, SERVER_KEY_DEST) else ""
        assert after_cert == before_cert, "Expected certificate to remain unchanged after mismatch"
        assert after_key == before_key, "Expected key to remain unchanged after mismatch"
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_missing_cert_key(server_host, xp2p_server_runner):
    missing_cert = PurePosixPath("/tmp/xp2p-missing-cert.pem")
    missing_key = PurePosixPath("/tmp/xp2p-missing-key.pem")
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62007",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        helpers.remove_path(server_host, missing_cert)
        helpers.remove_path(server_host, missing_key)

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cert",
            missing_cert.as_posix(),
            "--key",
            missing_key.as_posix(),
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected missing certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate file" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
        assert (
            "no such file" in combined
            or "not found" in combined
            or "file does not exist" in combined
        ), f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_requires_absolute_paths(server_host, xp2p_server_runner):
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62009",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cert",
            "tests/fixtures/tls/integration-cert.pem",
            "--key",
            "tests/fixtures/tls/integration-key.pem",
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected relative certificate/key paths to fail"
        combined = _combined_output(result).lower()
        assert "path must be absolute" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_win_store_not_implemented(server_host, xp2p_server_runner):
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62013",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        before_config = helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE)
        before_cert = helpers.file_sha256(server_host, SERVER_CERT_DEST) if helpers.path_exists(server_host, SERVER_CERT_DEST) else ""
        before_key = helpers.file_sha256(server_host, SERVER_KEY_DEST) if helpers.path_exists(server_host, SERVER_KEY_DEST) else ""

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cert-store",
            "MY",
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected win-store to be not implemented"
        combined = _combined_output(result).lower()
        assert "not implemented" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )

        after_config = helpers.read_text(server_host, helpers.SERVER_CONFIG_FILE)
        after_cert = helpers.file_sha256(server_host, SERVER_CERT_DEST) if helpers.path_exists(server_host, SERVER_CERT_DEST) else ""
        after_key = helpers.file_sha256(server_host, SERVER_KEY_DEST) if helpers.path_exists(server_host, SERVER_KEY_DEST) else ""
        assert after_config == before_config, "Expected desired config to remain unchanged after win-store error"
        assert after_cert == before_cert, "Expected certificate to remain unchanged after win-store error"
        assert after_key == before_key, "Expected key to remain unchanged after win-store error"
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_directory_paths(server_host, xp2p_server_runner):
    cert_dir = PurePosixPath("/tmp")
    key_dir = PurePosixPath("/tmp")
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62017",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cert",
            cert_dir.as_posix(),
            "--key",
            key_dir.as_posix(),
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected directory certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "is a directory" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_server_install_requires_force_when_state_exists(server_host, xp2p_server_runner):
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
        pass
