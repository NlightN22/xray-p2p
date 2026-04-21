import json
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable

import pytest

from tests.host.win import env as _env
from tests.host.win.flows import apply as apply_flow
from tests.host.win.flows.render import render_desired_xray_json

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_LIVE_XRAY_JSON = _env.CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME / "xray.json"
SERVER_LOGS_JSON = SERVER_CONFIG_DIR / "logs.json"
SERVER_CERT_DEST = SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = SERVER_CONFIG_DIR / "key.pem"
TLS_SERVER_DIR = _env.CONFIG_ROOT / "tls" / "server"
TLS_CERT_DEST = TLS_SERVER_DIR / "cert.pem"
TLS_KEY_DEST = TLS_SERVER_DIR / "key.pem"
SERVER_BIN_DIR = SERVER_INSTALL_DIR / "bin"
XRAY_BINARY = SERVER_BIN_DIR / "xray.exe"
SERVER_RUN_LOG = _env.LOGS_DIR / "xp2p-server-run.out"
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
        [SERVER_CONFIG_DIR, *SERVER_STATE_FILES, SERVER_RUN_LOG, SERVER_INSTALL_STATE],
    )


def _remote_path_exists(host, path: Path) -> bool:
    return _env.path_exists(host, path)


def _read_remote_text(host, path: Path) -> str:
    return _env.read_text(host, path)


def _read_remote_json(host, path: Path) -> dict:
    content = _read_remote_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


def _write_remote_text(host, path: Path, content: str) -> None:
    _env.write_text(host, path, content)


def _remove_remote_path(host, path: Path) -> None:
    _remove_remote_paths(host, [path])


def _remove_remote_paths(host, paths: Iterable[Path]) -> None:
    _env.remove_paths(host, paths)


def _expand_pending_targets(paths: Iterable[Path]) -> list[Path]:
    targets: list[Path] = []
    for path in paths:
        pending = _env.pending_candidate(path)
        targets.append(pending)
        if pending != path:
            targets.append(path)
    return targets


def _path_variants(path: Path) -> set[str]:
    raw = str(path)
    return {raw, raw.replace("\\", "/")}


def _expect_tls_paths() -> tuple[set[str], set[str]]:
    cert_paths = set()
    key_paths = set()
    for cert_path in (
        SERVER_CERT_DEST,
        TLS_CERT_DEST,
        _env.CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME / "cert.pem",
        _env.CONFIG_PENDING_ROOT / SERVER_CONFIG_DIR_NAME / "cert.pem",
        _env.CONFIG_LIVE_ROOT / "tls" / "server" / "cert.pem",
        _env.CONFIG_PENDING_ROOT / "tls" / "server" / "cert.pem",
    ):
        cert_paths.update(_path_variants(cert_path))
        cert_paths.update(_path_variants(_env.pending_candidate(cert_path)))

    for key_path in (
        SERVER_KEY_DEST,
        TLS_KEY_DEST,
        _env.CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME / "key.pem",
        _env.CONFIG_PENDING_ROOT / SERVER_CONFIG_DIR_NAME / "key.pem",
        _env.CONFIG_LIVE_ROOT / "tls" / "server" / "key.pem",
        _env.CONFIG_PENDING_ROOT / "tls" / "server" / "key.pem",
    ):
        key_paths.update(_path_variants(key_path))
        key_paths.update(_path_variants(_env.pending_candidate(key_path)))
    return cert_paths, key_paths


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Trojan inbound not found in configuration data")


def _ensure_live_xray(server_host, runner) -> None:
    if _env.path_exists(server_host, SERVER_LIVE_XRAY_JSON):
        return
    if not _env.service_exists(server_host, "xp2p-server"):
        pytest.skip("xp2p-server service is not registered; MSI install required.")
    runner("server", "service", "start", check=True)
    apply_flow.wait_for_apply_request_clear(server_host, timeout=90.0)
    runner("server", "service", "stop", check=True)
    deadline = time.time() + 30.0
    while time.time() < deadline:
        if _env.path_exists(server_host, SERVER_LIVE_XRAY_JSON):
            return
        time.sleep(1.0)
    pytest.fail(f"Live xray.json was not created at {SERVER_LIVE_XRAY_JSON}.")


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


def _primary_tls_files(data: dict) -> tuple[str, str]:
    trojan = _trojan_inbound(data)
    tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
    certificates = tls_settings.get("certificates", [])
    assert certificates, "Expected TLS certificates in configuration"
    primary = certificates[0] if isinstance(certificates, list) else {}
    cert_file = (primary.get("certificateFile") or "").strip()
    key_file = (primary.get("keyFile") or "").strip()
    assert cert_file, "certificateFile missing in TLS configuration"
    assert key_file, "keyFile missing in TLS configuration"
    return cert_file, key_file


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
        assert _remote_path_exists(server_host, _env.CONFIG_ROOT / "xp2p-server.toml")

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

        _ensure_live_xray(server_host, xp2p_server_runner)
        xray = _read_remote_json(server_host, SERVER_LIVE_XRAY_JSON)
        trojan = _trojan_inbound(xray)
        assert trojan.get("port") == 62001
        stream_settings = trojan.get("streamSettings", {})
        assert stream_settings.get("security") == "tls"
        tls_settings = stream_settings.get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        cert_value, key_value = _primary_tls_files(xray)
        expected_cert_paths, expected_key_paths = _expect_tls_paths()
        assert cert_value in expected_cert_paths
        assert key_value in expected_key_paths
        assert _env.path_exists(server_host, Path(cert_value)), f"Expected certificateFile to exist: {cert_value}"
        assert _env.path_exists(server_host, Path(key_value)), f"Expected keyFile to exist: {key_value}"
        remote_cert = _read_remote_text(server_host, Path(cert_value)).replace("\r\n", "\n").strip()
        remote_key = _read_remote_text(server_host, Path(key_value)).replace("\r\n", "\n").strip()
        assert remote_cert == cert_content.replace("\r\n", "\n").strip()
        assert remote_key == key_content.replace("\r\n", "\n").strip()

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

        updated_desired = render_desired_xray_json(xp2p_server_runner, role="server")
        updated_cert_value, _ = _primary_tls_files(updated_desired)
        updated_cert = _read_remote_text(server_host, Path(updated_cert_value)).replace("\r\n", "\n").strip()
        assert updated_cert != cert_content.replace("\r\n", "\n").strip(), (
            "Expected self-signed cert generation to overwrite the previous certificate."
        )

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

        _ensure_live_xray(server_host, xp2p_server_runner)
        updated_xray = _read_remote_json(server_host, SERVER_LIVE_XRAY_JSON)
        updated_trojan = _trojan_inbound(updated_xray)
        assert updated_trojan.get("port") == 62001
        updated_stream = updated_trojan.get("streamSettings", {})
        assert updated_stream.get("security") == "tls"
        updated_tls = updated_stream.get("tlsSettings", {})
        assert "allowInsecure" not in updated_tls
        updated_cert_value, updated_key_value = _primary_tls_files(updated_xray)
        assert updated_cert_value in expected_cert_paths
        assert updated_key_value in expected_key_paths
        updated_remote_cert = _read_remote_text(server_host, Path(updated_cert_value)).replace("\r\n", "\n").strip()
        updated_remote_key = _read_remote_text(server_host, Path(updated_key_value)).replace("\r\n", "\n").strip()
        assert updated_remote_cert == cert_content.replace("\r\n", "\n").strip()
        assert updated_remote_key == key_content.replace("\r\n", "\n").strip()
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

        _ensure_live_xray(server_host, xp2p_server_runner)
        xray = _read_remote_json(server_host, SERVER_LIVE_XRAY_JSON)
        trojan = _trojan_inbound(xray)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        cert_value, key_value = _primary_tls_files(xray)
        expected_cert_paths, expected_key_paths = _expect_tls_paths()
        assert cert_value in expected_cert_paths
        assert key_value in expected_key_paths
        assert _env.path_exists(server_host, Path(cert_value)), f"Expected certificateFile to exist: {cert_value}"
        assert _env.path_exists(server_host, Path(key_value)), f"Expected keyFile to exist: {key_value}"
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

        desired_xray = render_desired_xray_json(xp2p_server_runner, role="server")
        cert_value, key_value = _primary_tls_files(desired_xray)
        cert_path = Path(cert_value)
        key_path = Path(key_value)
        assert _remote_path_exists(server_host, cert_path), f"Expected certificateFile to exist: {cert_value}"
        assert _remote_path_exists(server_host, key_path), f"Expected keyFile to exist: {key_value}"

        cert_info = _decode_remote_certificate(server_host, cert_path)
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

        key_content = _read_remote_text(server_host, key_path)
        assert "BEGIN RSA PRIVATE KEY" in key_content

        _ensure_live_xray(server_host, xp2p_server_runner)
        xray = _read_remote_json(server_host, SERVER_LIVE_XRAY_JSON)
        trojan = _trojan_inbound(xray)
        stream_settings = trojan.get("streamSettings", {})
        assert stream_settings.get("security") == "tls"
        tls_settings = stream_settings.get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS configuration for self-signed certificate"
        expected_cert_paths, expected_key_paths = _expect_tls_paths()
        cert_ref = certificates[0]
        assert cert_ref.get("certificateFile") in expected_cert_paths
        assert cert_ref.get("keyFile") in expected_key_paths

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
        expected_cert_paths, expected_key_paths = _expect_tls_paths()
        assert any(f"Certificate: {path}" in state.stdout for path in expected_cert_paths)
        assert any(f"Key:         {path}" in state.stdout for path in expected_key_paths)
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
        desired_xray = render_desired_xray_json(xp2p_server_runner, role="server")
        _, key_value = _primary_tls_files(desired_xray)
        key_path = Path(key_value)
        assert _remote_path_exists(server_host, key_path), f"Expected generated key at {key_value}"

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
            str(key_path),
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

        before = render_desired_xray_json(
            xp2p_server_runner,
            role="server",
            config_path=str(_env.CONFIG_ROOT / "xp2p-server.toml"),
        )

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

        after = render_desired_xray_json(
            xp2p_server_runner,
            role="server",
            config_path=str(_env.CONFIG_ROOT / "xp2p-server.toml"),
        )
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
        xp2p_server_runner(
            "server",
            "mode",
            "proxy",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            check=True,
        )

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0

        assert _remote_path_exists(server_host, SERVER_RUN_LOG), (
            f"Expected log file {SERVER_RUN_LOG} to be created"
        )
        log_content = _read_remote_text(server_host, SERVER_RUN_LOG)
        assert log_content.strip(), "Expected xray-core to produce log output"
        assert "Failed to start" not in log_content
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
        _remove_remote_paths(server_host, [SERVER_RUN_LOG])


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

        targets = _expand_pending_targets([SERVER_CONFIG_DIR, *SERVER_STATE_FILES, SERVER_INSTALL_STATE])
        _remove_remote_paths(server_host, targets)
        assert not any(_remote_path_exists(server_host, path) for path in targets), (
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
        ]
        deadline = time.time() + 10.0
        missing: list[Path] = []
        while time.time() < deadline:
            missing = [path for path in expected_paths if not _remote_path_exists(server_host, path)]
            if not missing:
                break
            time.sleep(1.0)
        assert not missing, f"Expected server config files to be recreated: {missing}"
    finally:
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
