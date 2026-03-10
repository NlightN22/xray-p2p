import base64
import json
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable

import pytest

from tests.host.win import env as _env

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"
SERVER_LOGS_JSON = SERVER_CONFIG_DIR / "logs.json"
SERVER_OUTBOUNDS_JSON = SERVER_CONFIG_DIR / "outbounds.json"
SERVER_ROUTING_JSON = SERVER_CONFIG_DIR / "routing.json"
SERVER_CERT_DEST = SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = SERVER_CONFIG_DIR / "key.pem"
SERVER_BIN_DIR = SERVER_INSTALL_DIR / "bin"
XRAY_BINARY = SERVER_BIN_DIR / "xray.exe"
SERVER_LOG_RELATIVE = r"logs\server.err"
SERVER_LOG_FILE = _env.LOGS_DIR / "server.err"
SERVER_HOST_VALUE = "xp2p.test.local"
SERVER_INSTALL_STATE = _env.CONFIG_ROOT / "install-state-server.json"
SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]
FIXTURE_CERT = Path("tests/fixtures/tls/integration-cert.pem")
FIXTURE_KEY = Path("tests/fixtures/tls/integration-key.pem")
FIXTURE_CERT_GUEST = Path(r"C:\xp2p\tests\fixtures\tls\integration-cert.pem")
FIXTURE_KEY_GUEST = Path(r"C:\xp2p\tests\fixtures\tls\integration-key.pem")


def _cleanup_server_install(server_host, runner, msi_path: str) -> None:
    runner(
        "server",
        "remove",
        "--path",
        str(SERVER_INSTALL_DIR),
        "--ignore-missing",
    )
    _remove_remote_paths(
        server_host,
        [SERVER_CONFIG_DIR, *SERVER_STATE_FILES, SERVER_LOG_FILE, SERVER_INSTALL_STATE],
    )


def _remote_path_exists(host, path: Path) -> bool:
    result = _env.run_guest_script(
        host,
        "scripts/path_exists.ps1",
        force_stage=True,
        TargetPath=str(path),
    )
    if result.rc == 0:
        return True
    if result.rc == 3:
        return False
    if not (result.stdout or result.stderr):
        return False
    pytest.fail(
        f"Failed to check remote path {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _read_remote_text(host, path: Path) -> str:
    result = _env.run_guest_script(
        host,
        "scripts/read_file.ps1",
        Path=str(path),
    )
    if result.rc != 0:
        pytest.fail(
            f"Failed to read remote text {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def _read_remote_json(host, path: Path) -> dict:
    content = _read_remote_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


def _write_remote_text(host, path: Path, content: str) -> None:
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    result = _env.run_guest_script(
        host,
        "scripts/write_file.ps1",
        Path=str(path),
        ContentBase64=encoded,
    )
    if result.rc != 0:
        pytest.fail(
            f"Failed to write remote text {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_remote_path(host, path: Path) -> None:
    _remove_remote_paths(host, [path])


def _remove_remote_paths(host, paths: Iterable[Path]) -> None:
    payload = base64.b64encode(
        json.dumps([str(path) for path in paths]).encode("utf-8")
    ).decode("ascii")
    result = _env.run_guest_script(
        host,
        "scripts/remove_paths.ps1",
        PathsBase64=payload,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to remove remote paths.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _expect_tls_paths() -> tuple[str, str]:
    expected_cert = str(SERVER_CERT_DEST).replace("\\", "/")
    expected_key = str(SERVER_KEY_DEST).replace("\\", "/")
    return expected_cert, expected_key


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Trojan inbound not found in configuration data")


def _decode_remote_certificate(host, path: Path) -> dict:
    result = _env.run_guest_script(
        host,
        "scripts/get_certificate_info.ps1",
        Path=str(path),
    )
    assert result.rc == 0, (
        f"Failed to decode certificate {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    return json.loads(result.stdout)


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".strip()


@pytest.mark.host
@pytest.mark.win
def test_server_install_uses_provided_certificate_and_force_overwrites(
    server_host, xp2p_server_runner, xp2p_msi_path
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    cert_source = Path(r"C:\Users\vagrant\AppData\Local\Temp\xp2p-server-cert.pem")
    key_source = Path(r"C:\Users\vagrant\AppData\Local\Temp\xp2p-server-key.pem")
    cert_content = FIXTURE_CERT.read_text(encoding="utf-8")
    key_content = FIXTURE_KEY.read_text(encoding="utf-8")
    try:
        _write_remote_text(server_host, cert_source, cert_content)
        _write_remote_text(server_host, key_source, key_content)

        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62001",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        assert _remote_path_exists(server_host, XRAY_BINARY), (
            f"Expected xray binary at {XRAY_BINARY}"
        )
        for config_path in (
            SERVER_INBOUNDS,
            SERVER_LOGS_JSON,
            SERVER_OUTBOUNDS_JSON,
            SERVER_ROUTING_JSON,
        ):
            assert _remote_path_exists(server_host, config_path), (
                f"Expected config file {config_path}"
            )

        xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--cert",
            str(cert_source),
            "--key",
            str(key_source),
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        inbounds_data = _read_remote_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds_data)
        assert trojan.get("port") == 62001
        stream_settings = trojan.get("streamSettings", {})
        assert stream_settings.get("security") == "tls"
        tls_settings = stream_settings.get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates in configuration"
        expected_cert = str(cert_source).replace("\\", "/")
        expected_key = str(key_source).replace("\\", "/")
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == expected_cert
        assert primary_cert.get("keyFile") == expected_key

        _write_remote_text(server_host, cert_source, cert_content)
        _write_remote_text(server_host, key_source, key_content)

        xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--cert",
            str(cert_source),
            "--key",
            str(key_source),
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        updated_inbounds = _read_remote_json(server_host, SERVER_INBOUNDS)
        updated_trojan = _trojan_inbound(updated_inbounds)
        assert updated_trojan.get("port") == 62001
        updated_stream = updated_trojan.get("streamSettings", {})
        assert updated_stream.get("security") == "tls"
        updated_tls = updated_stream.get("tlsSettings", {})
        assert "allowInsecure" not in updated_tls
        updated_certificates = updated_tls.get("certificates", [])
        assert updated_certificates, "Expected TLS certificates after certificate update"
        updated_primary = updated_certificates[0]
        assert updated_primary.get("certificateFile") == expected_cert
        assert updated_primary.get("keyFile") == expected_key
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
        _remove_remote_paths(server_host, [cert_source, key_source])


@pytest.mark.host
@pytest.mark.win
def test_server_install_uses_path_certificate_source(server_host, xp2p_server_runner, xp2p_msi_path):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    cert_source = FIXTURE_CERT_GUEST
    key_source = FIXTURE_KEY_GUEST
    try:
        assert _remote_path_exists(server_host, cert_source), (
            f"Expected fixture certificate at {cert_source}"
        )
        assert _remote_path_exists(server_host, key_source), (
            f"Expected fixture key at {key_source}"
        )

        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62003",
            "--host",
            SERVER_HOST_VALUE,
            "--cert",
            str(cert_source),
            "--key",
            str(key_source),
            "--force",
            check=True,
        )

        inbounds_data = _read_remote_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds_data)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates in configuration"
        primary_cert = certificates[0]
        expected_cert, expected_key = _expect_tls_paths()
        assert primary_cert.get("certificateFile") == expected_cert
        assert primary_cert.get("keyFile") == expected_key

        assert _remote_path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to be copied"
        assert _remote_path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to be copied"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_install_generates_self_signed_certificate(
    server_host, xp2p_server_runner, xp2p_msi_path
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62015",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        assert _remote_path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to exist"
        assert _remote_path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to exist"

        cert_info = _decode_remote_certificate(server_host, SERVER_CERT_DEST)
        subject_cn = cert_info.get("SubjectCN")
        assert subject_cn == SERVER_HOST_VALUE, (
            f"Expected CN={SERVER_HOST_VALUE}, got {subject_cn}"
        )

        san_entries = cert_info.get("SubjectAltName", [])
        san_hosts = {
            entry["Value"]
            for entry in san_entries
            if entry.get("Type", "").lower() == "dns name"
        }
        assert SERVER_HOST_VALUE in san_hosts, (
            f"Expected DNS SAN entries to include {SERVER_HOST_VALUE}, got {san_hosts}"
        )

        not_after_str = cert_info.get("NotAfter")
        assert not_after_str, "Certificate notAfter missing"
        not_after = datetime.fromisoformat(
            not_after_str.replace("Z", "+00:00")
        ).astimezone(timezone.utc)
        now = datetime.now(timezone.utc)
        assert not_after - now > timedelta(days=9 * 365), (
            f"Expected certificate validity close to 10 years, got {not_after - now}"
        )

        key_content = _read_remote_text(server_host, SERVER_KEY_DEST)
        assert "BEGIN RSA PRIVATE KEY" in key_content

        inbounds_data = _read_remote_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds_data)
        stream_settings = trojan.get("streamSettings", {})
        assert stream_settings.get("security") == "tls"
        tls_settings = stream_settings.get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS configuration for self-signed certificate"
        expected_cert, expected_key = _expect_tls_paths()
        cert_ref = certificates[0]
        assert cert_ref.get("certificateFile") == expected_cert
        assert cert_ref.get("keyFile") == expected_key

        state = xp2p_server_runner(
            "server",
            "cert",
            "state",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            check=False,
        )
        assert state.rc == 0, (
            f"Expected cert state to succeed, rc={state.rc}\n"
            f"STDOUT:\n{state.stdout}\nSTDERR:\n{state.stderr}"
        )
        assert "Status:      OK" in state.stdout
        assert f"Subject:     CN={SERVER_HOST_VALUE}" in state.stdout
        assert f"Certificate: {SERVER_CERT_DEST}" in state.stdout
        assert f"Key:         {SERVER_KEY_DEST}" in state.stdout
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_cert_set_rejects_mismatched_cert_key(server_host, xp2p_server_runner, xp2p_msi_path):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62005",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        assert _remote_path_exists(server_host, SERVER_KEY_DEST), (
            f"Expected generated key at {SERVER_KEY_DEST}"
        )

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--cert",
            str(FIXTURE_CERT_GUEST),
            "--key",
            str(SERVER_KEY_DEST),
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected mismatched certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate and key do not match" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_cert_set_rejects_missing_cert_key(server_host, xp2p_server_runner, xp2p_msi_path):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    missing_cert = Path(r"C:\Windows\Temp\xp2p-missing-cert.pem")
    missing_key = Path(r"C:\Windows\Temp\xp2p-missing-key.pem")
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62007",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        _remove_remote_paths(server_host, [missing_cert, missing_key])

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--cert",
            str(missing_cert),
            "--key",
            str(missing_key),
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected missing certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate file" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
        assert (
            "cannot find the file" in combined
            or "no such file" in combined
            or "file does not exist" in combined
        ), f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_cert_set_requires_absolute_paths(server_host, xp2p_server_runner, xp2p_msi_path):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62009",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--cert",
            r"tests\fixtures\tls\integration-cert.pem",
            "--key",
            r"tests\fixtures\tls\integration-key.pem",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected relative certificate/key paths to fail"
        combined = _combined_output(result).lower()
        assert "path must be absolute" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_cert_set_win_store_not_implemented(server_host, xp2p_server_runner, xp2p_msi_path):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62013",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        before = _read_remote_text(server_host, SERVER_INBOUNDS)

        result = xp2p_server_runner(
            "server",
            "cert",
            "set",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
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

        after = _read_remote_text(server_host, SERVER_INBOUNDS)
        assert after == before, "Expected config to remain unchanged after win-store error"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_run_starts_xray_core(
    server_host, xp2p_server_runner, xp2p_server_run_factory, xp2p_msi_path
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62011",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME, SERVER_LOG_RELATIVE
        ) as session:
            assert session["pid"] > 0

        assert _remote_path_exists(server_host, SERVER_LOG_FILE), (
            f"Expected log file {SERVER_LOG_FILE} to be created"
        )
        log_content = _read_remote_text(server_host, SERVER_LOG_FILE)
        assert log_content.strip(), "Expected xray-core to produce log output"
        assert "Failed to start" not in log_content
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
        _remove_remote_paths(server_host, [SERVER_LOG_FILE])


@pytest.mark.host
@pytest.mark.win
def test_server_install_requires_force_when_state_exists(
    server_host, xp2p_server_runner, xp2p_msi_path
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62100",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        result = xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62101",
            "--host",
            SERVER_HOST_VALUE,
            check=False,
        )
        assert result.rc != 0, "Expected second install without --force to fail when state file exists"
        combined = f"{result.stdout}\n{result.stderr}".strip().lower()
        assert "server files already present" in combined, f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_server_install_succeeds_without_state_marker(
    server_host, xp2p_server_runner, xp2p_msi_path
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62110",
            "--host",
            SERVER_HOST_VALUE,
            "--force",
            check=True,
        )

        _remove_remote_paths(server_host, [SERVER_CONFIG_DIR, *SERVER_STATE_FILES, SERVER_INSTALL_STATE])
        assert not any(
            _remote_path_exists(server_host, path)
            for path in [SERVER_CONFIG_DIR, *SERVER_STATE_FILES, SERVER_INSTALL_STATE]
        ), (
            "Expected server state files to be removed before re-install"
        )

        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62111",
            "--host",
            SERVER_HOST_VALUE,
            check=True,
        )

        expected_paths = [
            _env.CONFIG_ROOT / "xp2p-server.toml",
            SERVER_INSTALL_STATE,
        ]
        deadline = time.time() + 10.0
        missing: list[Path] = []
        while time.time() < deadline:
            missing = [path for path in expected_paths if not _remote_path_exists(server_host, path)]
            if not missing:
                break
            time.sleep(1.0)
        assert not missing, f"Expected server config/state files to be recreated: {missing}"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
