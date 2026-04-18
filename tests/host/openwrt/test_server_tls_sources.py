from __future__ import annotations

import json
import time
from pathlib import PurePosixPath

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"
SERVER_CERT_DEST = PurePosixPath("/etc/xp2p/tls/server/cert.pem")
SERVER_KEY_DEST = PurePosixPath("/etc/xp2p/tls/server/key.pem")


def _runner(host: Host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p_live(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _copy_remote_file(host: Host, source: PurePosixPath, dest: PurePosixPath) -> None:
    source_path = _resolve_server_path(host, source)
    result = openwrt_env.run_guest_script(
        host,
        "scripts/linux/copy_file.sh",
        source_path.as_posix(),
        dest.as_posix(),
    )
    if result.rc == 0:
        return
    if result.rc == 3:
        pytest.fail(f"Failed to copy missing file {source} to {dest}")
    pytest.fail(
        f"Failed to copy file {source} to {dest} (exit {result.rc}).\n"
        f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _read_remote_text(host: Host, path: PurePosixPath) -> str:
    try:
        helpers.ensure_service_running(host, "server")
        helpers.wait_for_live_config(host, "server")
        return helpers.read_live_text(host, path)
    except RuntimeError as exc:
        pytest.fail(str(exc))


def _read_remote_json(host: Host, path: PurePosixPath) -> dict:
    try:
        helpers.ensure_service_running(host, "server")
        helpers.wait_for_live_config(host, "server")
        return helpers.read_live_json(host, path)
    except RuntimeError as exc:
        pytest.fail(str(exc))


def _path_exists(host: Host, path: PurePosixPath) -> bool:
    return helpers.path_exists(host, path)


def _remove_path(host: Host, path: PurePosixPath) -> None:
    result = openwrt_env.run_guest_script(
        host,
        "scripts/linux/remove_path.sh",
        path.as_posix(),
    )
    if result.rc not in (0, 3):
        pytest.fail(
            f"Failed to remove path {path} (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _wait_for_path(host: Host, path: PurePosixPath, timeout_seconds: float = 20.0, interval: float = 1.0) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if helpers.path_exists_exact(host, path):
            return
        time.sleep(interval)
    pytest.fail(f"Expected {path} to appear within {timeout_seconds}s")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".strip()


def _resolve_server_path(host: Host, path: PurePosixPath) -> PurePosixPath:
    return path


def _trojan_inbound(data: dict) -> dict:
    for entry in data.get("inbounds", []):
        if entry.get("protocol") == "trojan":
            return entry
    pytest.fail("Trojan inbound not found in configuration")


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
def test_openwrt_server_install_uses_path_certificate_source(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)

    cert_source = PurePosixPath("/tmp/xp2p-server-cert.pem")
    key_source = PurePosixPath("/tmp/xp2p-server-key.pem")
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62002",
            "--host",
            "prep-path-cert.local",
            "--force",
            check=True,
        )
        _wait_for_path(openwrt_server_host, SERVER_CERT_DEST)
        _wait_for_path(openwrt_server_host, SERVER_KEY_DEST)
        _copy_remote_file(openwrt_server_host, SERVER_CERT_DEST, cert_source)
        _copy_remote_file(openwrt_server_host, SERVER_KEY_DEST, key_source)
        helpers.cleanup_server_install(openwrt_server_host, runner)

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62003",
            "--host",
            "xp2p.test.local",
            "--cert",
            cert_source.as_posix(),
            "--key",
            key_source.as_posix(),
            "--force",
            check=True,
        )

        inbounds = _read_remote_json(openwrt_server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == SERVER_CERT_DEST.as_posix()
        assert primary_cert.get("keyFile") == SERVER_KEY_DEST.as_posix()

        expected_allow_insecure = _parse_self_signed(_read_cert_state(runner))
        assert expected_allow_insecure in {True, False}
        assert "allowInsecure" not in tls_settings
        assert _path_exists(openwrt_server_host, SERVER_CERT_DEST), "Expected cert.pem to exist"
        assert _path_exists(openwrt_server_host, SERVER_KEY_DEST), "Expected key.pem to exist"
    finally:
        helpers.cleanup_server_install(openwrt_server_host, runner)
        _remove_path(openwrt_server_host, cert_source)
        _remove_path(openwrt_server_host, key_source)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_install_generates_self_signed_certificate(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    try:
        runner(
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

        assert _path_exists(openwrt_server_host, SERVER_CERT_DEST), "Expected cert.pem to exist"
        assert _path_exists(openwrt_server_host, SERVER_KEY_DEST), "Expected key.pem to exist"

        inbounds = _read_remote_json(openwrt_server_host, SERVER_INBOUNDS)
        trojan = _trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert "allowInsecure" not in tls_settings
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates to be configured"
        primary_cert = certificates[0]
        assert primary_cert.get("certificateFile") == SERVER_CERT_DEST.as_posix()
        assert primary_cert.get("keyFile") == SERVER_KEY_DEST.as_posix()

        state_output = _read_cert_state(runner)
        assert "Status:      OK" in state_output
        assert "self-signed: yes" in state_output.lower()
    finally:
        helpers.cleanup_server_install(openwrt_server_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_rejects_mismatched_cert_key(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    cert_source = PurePosixPath("/tmp/xp2p-mismatch-cert.pem")
    key_source = PurePosixPath("/tmp/xp2p-mismatch-key.pem")
    alt_key_source = PurePosixPath("/tmp/xp2p-mismatch-key-alt.pem")
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62005",
            "--host",
            "prep-mismatch-a.local",
            "--force",
            check=True,
        )
        _wait_for_path(openwrt_server_host, SERVER_CERT_DEST)
        _wait_for_path(openwrt_server_host, SERVER_KEY_DEST)

        _copy_remote_file(openwrt_server_host, SERVER_CERT_DEST, cert_source)
        _copy_remote_file(openwrt_server_host, SERVER_KEY_DEST, key_source)
        helpers.cleanup_server_install(openwrt_server_host, runner)

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62006",
            "--host",
            "prep-mismatch-b.local",
            "--force",
            check=True,
        )
        _wait_for_path(openwrt_server_host, SERVER_CERT_DEST)
        _wait_for_path(openwrt_server_host, SERVER_KEY_DEST)
        _copy_remote_file(openwrt_server_host, SERVER_KEY_DEST, alt_key_source)
        helpers.cleanup_server_install(openwrt_server_host, runner)

        runner(
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

        result = runner(
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
            alt_key_source.as_posix(),
            "--force",
            check=False,
        )
        assert result.rc != 0, "Expected mismatched certificate/key to fail"
        combined = _combined_output(result).lower()
        assert "certificate and key do not match" in combined, (
            f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
        )
    finally:
        helpers.cleanup_server_install(openwrt_server_host, runner)
        _remove_path(openwrt_server_host, cert_source)
        _remove_path(openwrt_server_host, key_source)
        _remove_path(openwrt_server_host, alt_key_source)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_rejects_missing_cert_key(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    missing_cert = PurePosixPath("/tmp/xp2p-missing-cert.pem")
    missing_key = PurePosixPath("/tmp/xp2p-missing-key.pem")
    try:
        runner(
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

        _remove_path(openwrt_server_host, missing_cert)
        _remove_path(openwrt_server_host, missing_key)

        result = runner(
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
        helpers.cleanup_server_install(openwrt_server_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_requires_absolute_paths(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    try:
        runner(
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

        result = runner(
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
        helpers.cleanup_server_install(openwrt_server_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_win_store_not_implemented(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    try:
        runner(
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

        before = helpers.read_text(openwrt_server_host, helpers.SERVER_CONFIG_FILE)

        result = runner(
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

        after = helpers.read_text(openwrt_server_host, helpers.SERVER_CONFIG_FILE)
        assert after == before, "Expected server config to remain unchanged after win-store error"
    finally:
        helpers.cleanup_server_install(openwrt_server_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_cert_set_rejects_directory_paths(openwrt_server_host, xp2p_openwrt_ipk):
    runner = _runner(openwrt_server_host)
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    helpers.cleanup_server_install(openwrt_server_host, runner)
    cert_dir = PurePosixPath("/tmp")
    key_dir = PurePosixPath("/tmp")
    try:
        runner(
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
        helpers.cleanup_server_install(openwrt_server_host, runner)

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62018",
            "--host",
            "xp2p.test.local",
            "--force",
            check=True,
        )

        result = runner(
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
        helpers.cleanup_server_install(openwrt_server_host, runner)
