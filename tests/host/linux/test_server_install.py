from __future__ import annotations

from pathlib import Path, PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers

SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"
SERVER_OUTBOUNDS = helpers.SERVER_CONFIG_DIR / "outbounds.json"
SERVER_LOGS_JSON = helpers.SERVER_CONFIG_DIR / "logs.json"
SERVER_ROUTING_JSON = helpers.SERVER_CONFIG_DIR / "routing.json"
SERVER_CERT_DEST = helpers.SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = helpers.SERVER_CONFIG_DIR / "key.pem"
FIXTURE_CERT = Path("tests/fixtures/tls/integration-cert.pem")
FIXTURE_KEY = Path("tests/fixtures/tls/integration-key.pem")


def _cleanup(server_host, xp2p_server_runner) -> None:
    helpers.cleanup_server_install(server_host, xp2p_server_runner)


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Trojan inbound not found in configuration")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".strip()


def _read_cert_state(runner) -> str:
    result = runner(
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
    return result.stdout or ""


def _parse_self_signed(state_output: str) -> bool:
    for line in (state_output or "").splitlines():
        if line.strip().lower().startswith("self-signed:"):
            value = line.split(":", 1)[1].strip().lower()
            if value in {"yes", "true"}:
                return True
            if value in {"no", "false"}:
                return False
    pytest.fail(f"Self-signed line not found in cert state output:\n{state_output}")


@pytest.mark.host
@pytest.mark.linux
def test_server_install_uses_provided_certificate_and_force_overwrites(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
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
            SERVER_INBOUNDS,
            SERVER_OUTBOUNDS,
            SERVER_LOGS_JSON,
            SERVER_ROUTING_JSON,
        ):
            assert helpers.path_exists(server_host, config_path), f"Missing config file {config_path}"

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

        inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds)
        assert trojan.get("port") == 62001
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == cert_source.as_posix()
        assert primary_cert.get("keyFile") == key_source.as_posix()
        expected_allow_insecure = _parse_self_signed(_read_cert_state(xp2p_server_runner))
        assert bool(tls_settings.get("allowInsecure")) is expected_allow_insecure
        assert helpers.path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to exist in config-server"
        assert helpers.path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to exist in config-server"
    finally:
        helpers.remove_path(server_host, cert_source)
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_install_generates_self_signed_certificate(server_host, xp2p_server_runner):
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
            "62015",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        assert helpers.path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to exist"
        assert helpers.path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to exist"

        inbounds = helpers.read_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert tls_settings.get("allowInsecure") is True
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == SERVER_CERT_DEST.as_posix()
        assert primary_cert.get("keyFile") == SERVER_KEY_DEST.as_posix()

        state_output = _read_cert_state(xp2p_server_runner)
        assert "Status:      OK" in state_output
        assert "self-signed: yes" in state_output.lower()
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_mismatched_cert_key(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    cert_source = PurePosixPath("/tmp/xp2p-mismatch-cert.pem")
    cert_content = FIXTURE_CERT.read_text(encoding="utf-8")
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

        assert helpers.path_exists(server_host, SERVER_KEY_DEST), (
            f"Expected generated key at {SERVER_KEY_DEST}"
        )
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
            SERVER_KEY_DEST.as_posix(),
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected mismatched certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate and key do not match" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_missing_cert_key(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
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
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_requires_absolute_paths(server_host, xp2p_server_runner):
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
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_win_store_not_implemented(server_host, xp2p_server_runner):
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
            "62013",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        before = helpers.read_text(server_host, SERVER_INBOUNDS)

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

        after = helpers.read_text(server_host, SERVER_INBOUNDS)
        assert after == before, "Expected config to remain unchanged after win-store error"
    finally:
        _cleanup(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_cert_set_rejects_directory_paths(server_host, xp2p_server_runner):
    _cleanup(server_host, xp2p_server_runner)
    cert_dir = PurePosixPath("/tmp/xp2p-cert-dir")
    key_dir = PurePosixPath("/tmp/xp2p-key-dir")
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

        helpers.write_text(server_host, cert_dir / "dummy.txt", "cert")
        helpers.write_text(server_host, key_dir / "dummy.txt", "key")

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
        helpers.remove_path(server_host, cert_dir)
        helpers.remove_path(server_host, key_dir)
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
