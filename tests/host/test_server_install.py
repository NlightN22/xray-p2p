import base64
import json
import os
import ssl
import tempfile
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest

from tests.host import _env

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = SERVER_INSTALL_DIR / SERVER_CONFIG_DIR_NAME
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"
SERVER_LOGS_JSON = SERVER_CONFIG_DIR / "logs.json"
SERVER_OUTBOUNDS_JSON = SERVER_CONFIG_DIR / "outbounds.json"
SERVER_ROUTING_JSON = SERVER_CONFIG_DIR / "routing.json"
SERVER_CERT_DEST = SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = SERVER_CONFIG_DIR / "key.pem"
SERVER_BIN_DIR = SERVER_INSTALL_DIR / "bin"
XRAY_BINARY = SERVER_BIN_DIR / "xray.exe"
SERVER_LOG_RELATIVE = r"logs\server.err"
SERVER_LOG_FILE = SERVER_INSTALL_DIR / SERVER_LOG_RELATIVE
SERVER_HOST_VALUE = "xp2p.test.local"


def _cleanup_server_install(runner) -> None:
    runner(
        "server",
        "remove",
        "--path",
        str(SERVER_INSTALL_DIR),
        "--ignore-missing",
    )
    _env.prepare_server_program_files_install()


def _remote_path_exists(host, path: Path) -> bool:
    quoted = _env.ps_quote(str(path))
    script = f"if (Test-Path {quoted}) {{ exit 0 }} else {{ exit 3 }}"
    result = _env.run_powershell(host, script)
    return result.rc == 0


def _read_remote_text(host, path: Path) -> str:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Raw {quoted}
"""
    result = _env.run_powershell(host, script)
    assert result.rc == 0, (
        f"Failed to read remote text {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    return result.stdout


def _read_remote_json(host, path: Path) -> dict:
    content = _read_remote_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


def _write_remote_text(host, path: Path, content: str) -> None:
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    target = _env.ps_quote(str(path))
    parent = _env.ps_quote(str(path.parent))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {parent})) {{
    New-Item -ItemType Directory -Path {parent} -Force | Out-Null
}}
$data = [System.Convert]::FromBase64String('{encoded}')
$text = [System.Text.Encoding]::UTF8.GetString($data)
[System.IO.File]::WriteAllText({target}, $text)
"""
    result = _env.run_powershell(host, script)
    assert result.rc == 0, (
        f"Failed to write remote text {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _remove_remote_path(host, path: Path) -> None:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (Test-Path {quoted}) {{
    Remove-Item {quoted} -Force -Recurse -ErrorAction SilentlyContinue
}}
"""
    _env.run_powershell(host, script)


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
    pem_text = _read_remote_text(host, path)
    with tempfile.NamedTemporaryFile("w", delete=False, suffix=".pem") as tmp:
        tmp.write(pem_text)
        tmp.flush()
        tmp_path = tmp.name
    try:
        cert_info = ssl._ssl._test_decode_cert(tmp_path)
    finally:
        os.unlink(tmp_path)
    return cert_info


@pytest.mark.host
def test_server_install_uses_provided_certificate_and_force_overwrites(
    server_host, xp2p_server_runner
):
    _cleanup_server_install(xp2p_server_runner)
    cert_source = Path(r"C:\Users\vagrant\AppData\Local\Temp\xp2p-server-cert.pem")
    key_source = Path(r"C:\Users\vagrant\AppData\Local\Temp\xp2p-server-key.pem")
    first_cert_content = "CERTIFICATE-DATA-ONE"
    second_cert_content = "CERTIFICATE-DATA-TWO"
    first_key_content = "KEY-DATA-ONE"
    second_key_content = "KEY-DATA-TWO"
    try:
        _write_remote_text(server_host, cert_source, first_cert_content)
        _write_remote_text(server_host, key_source, first_key_content)

        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62001",
            "--cert",
            str(cert_source),
            "--key",
            str(key_source),
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

        inbounds_data = _read_remote_json(server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds_data)
        assert trojan.get("port") == 62001
        stream_settings = trojan.get("streamSettings", {})
        assert stream_settings.get("security") == "tls"
        tls_settings = stream_settings.get("tlsSettings", {})
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates in configuration"
        expected_cert, expected_key = _expect_tls_paths()
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == expected_cert
        assert primary_cert.get("keyFile") == expected_key

        _write_remote_text(server_host, cert_source, second_cert_content)
        _write_remote_text(server_host, key_source, second_key_content)

        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62005",
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
        assert updated_trojan.get("port") == 62005
        updated_stream = updated_trojan.get("streamSettings", {})
        assert updated_stream.get("security") == "tls"
        updated_tls = updated_stream.get("tlsSettings", {})
        updated_certificates = updated_tls.get("certificates", [])
        assert updated_certificates, "Expected TLS certificates after force reinstall"
        updated_primary = updated_certificates[0]
        assert updated_primary.get("certificateFile") == expected_cert
        assert updated_primary.get("keyFile") == expected_key

        cert_content = _read_remote_text(server_host, SERVER_CERT_DEST)
        key_content = _read_remote_text(server_host, SERVER_KEY_DEST)
        assert second_cert_content in cert_content
        assert second_key_content in key_content
    finally:
        _cleanup_server_install(xp2p_server_runner)
        _remove_remote_path(server_host, cert_source)
        _remove_remote_path(server_host, key_source)


@pytest.mark.host
def test_server_install_generates_self_signed_certificate(server_host, xp2p_server_runner):
    _cleanup_server_install(xp2p_server_runner)
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

        assert _remote_path_exists(server_host, SERVER_CERT_DEST), "Expected cert.pem to exist"
        assert _remote_path_exists(server_host, SERVER_KEY_DEST), "Expected key.pem to exist"

        cert_info = _decode_remote_certificate(server_host, SERVER_CERT_DEST)
        subject = dict(cert_info.get("subject", []))
        assert subject.get("commonName") == SERVER_HOST_VALUE, (
            f"Expected CN={SERVER_HOST_VALUE}, got {subject}"
        )

        san_entries = cert_info.get("subjectAltName", [])
        san_hosts = {value for kind, value in san_entries if kind.lower() == "dns"}
        assert SERVER_HOST_VALUE in san_hosts, (
            f"Expected DNS SAN entries to include {SERVER_HOST_VALUE}, got {san_hosts}"
        )

        not_after_str = cert_info.get("notAfter")
        assert not_after_str, "Certificate notAfter missing"
        not_after = datetime.strptime(not_after_str, "%b %d %H:%M:%S %Y GMT").replace(
            tzinfo=timezone.utc
        )
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
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS configuration for self-signed certificate"
        expected_cert, expected_key = _expect_tls_paths()
        cert_ref = certificates[0]
        assert cert_ref.get("certificateFile") == expected_cert
        assert cert_ref.get("keyFile") == expected_key
    finally:
        _cleanup_server_install(xp2p_server_runner)


@pytest.mark.host
def test_server_run_starts_xray_core(
    server_host, xp2p_server_runner, xp2p_server_run_factory
):
    _cleanup_server_install(xp2p_server_runner)
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
        _cleanup_server_install(xp2p_server_runner)
        _remove_remote_path(server_host, SERVER_LOG_FILE)
