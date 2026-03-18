from __future__ import annotations

import json
import time
from pathlib import Path, PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

PINNED_JSON = Path("go/internal/xray/pinned.json")
SERVER_HOSTNAME = "pinned-version.local"
SERVER_PORT = 62101
RUN_LOG_SERVER = PurePosixPath("/tmp/xp2p-server-run.log")
RUN_LOG_CLIENT = PurePosixPath("/tmp/xp2p-client-run.log")
XRAY_BACKUP = PurePosixPath("/etc/xp2p/bin/xray.pinned.bak")


def _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner) -> None:
    for runner in (xp2p_server_runner, xp2p_client_runner):
        runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--ignore-missing",
        )
        runner(
            "server",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--ignore-missing",
        )
    linux_env.kill_xp2p_processes(server_host)
    linux_env.kill_xp2p_processes(client_host)
    linux_env.remove_path(server_host, RUN_LOG_SERVER)
    linux_env.remove_path(client_host, RUN_LOG_CLIENT)


def _pinned_version() -> str:
    payload = json.loads(PINNED_JSON.read_text(encoding="utf-8"))
    version = (payload or {}).get("version", "").strip()
    if not version:
        pytest.fail("Pinned xray version is empty")
    return version


def _mismatch_version(pinned: str) -> str:
    candidate = "".join("0" if ch.isdigit() else ch for ch in pinned)
    if candidate == pinned:
        last = pinned[-1]
        replacement = "1" if last != "1" else "2"
        candidate = pinned[:-1] + replacement
    if candidate == pinned or len(candidate) != len(pinned):
        pytest.fail(f"Unable to generate mismatch version from {pinned!r}")
    return candidate


def _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner) -> dict[str, str]:
    server_install = xp2p_server_runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        str(SERVER_PORT),
        "--host",
        SERVER_HOSTNAME,
        "--force",
        check=True,
    )
    credential = helpers.extract_trojan_credential(server_install.stdout or "")
    xp2p_client_runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--link",
        credential["link"],
        "--force",
        check=True,
    )
    return credential


def _patch_xray(host, replacement: str) -> None:
    linux_env.kill_xp2p_processes(host)
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/wrap_xray_version.sh",
        helpers.XRAY_BINARY.as_posix(),
        XRAY_BACKUP.as_posix(),
        replacement,
    )
    if result.rc != 0:
        raise RuntimeError(
            "Failed to wrap xray version.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _restore_xray(host) -> None:
    linux_env.kill_xp2p_processes(host)
    if linux_env.path_exists(host, XRAY_BACKUP):
        linux_env.run_guest_script(
            host,
            "scripts/linux/copy_file.sh",
            XRAY_BACKUP.as_posix(),
            helpers.XRAY_BINARY.as_posix(),
        )
        linux_env.remove_path(host, XRAY_BACKUP)


def _assert_mismatch_logged(host, role: str) -> None:
    log_path = RUN_LOG_SERVER if role == "server" else RUN_LOG_CLIENT
    content = linux_env.read_text(host, log_path).lower()
    assert "xray version mismatch" in content, (
        f"Expected version mismatch in {role} run log:\n{content}"
    )


def _assert_mismatch_warned(host, role: str) -> None:
    log_path = RUN_LOG_SERVER if role == "server" else RUN_LOG_CLIENT
    content = linux_env.read_text(host, log_path).lower()
    assert "xray version mismatch allowed" in content, (
        f"Expected mismatch warning in {role} run log:\n{content}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_xray_pinned_version_allows_matching(
    server_host, client_host, xp2p_server_runner, xp2p_client_runner
):
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        with linux_env.xp2p_run_session(
            server_host,
            "server",
            helpers.INSTALL_ROOT,
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ) as server_session:
            assert server_session["pid"] > 0
            with linux_env.xp2p_run_session(
                client_host,
                "client",
                helpers.INSTALL_ROOT,
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
            ) as client_session:
                assert client_session["pid"] > 0
    finally:
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_xray_pinned_version_rejects_mismatch(
    server_host, client_host, xp2p_server_runner, xp2p_client_runner
):
    pinned = _pinned_version()
    mismatch = _mismatch_version(pinned)
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    linux_env.remove_path(server_host, RUN_LOG_SERVER)
    linux_env.remove_path(client_host, RUN_LOG_CLIENT)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _patch_xray(server_host, mismatch)
        _patch_xray(client_host, mismatch)

        server_result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/start_xp2p_run_with_env.sh",
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE.as_posix(),
            "0",
            "0",
        )
        assert server_result.rc != 0, "Expected server run to fail with mismatched xray"
        _assert_mismatch_logged(server_host, "server")

        client_result = linux_env.run_guest_script(
            client_host,
            "scripts/linux/start_xp2p_run_with_env.sh",
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE.as_posix(),
            "0",
            "0",
        )
        assert client_result.rc != 0, "Expected client run to fail with mismatched xray"
        _assert_mismatch_logged(client_host, "client")
    finally:
        _restore_xray(server_host)
        _restore_xray(client_host)
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_xray_pinned_version_allows_override(
    server_host, client_host, xp2p_server_runner, xp2p_client_runner
):
    pinned = _pinned_version()
    mismatch = _mismatch_version(pinned)
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    linux_env.remove_path(server_host, RUN_LOG_SERVER)
    linux_env.remove_path(client_host, RUN_LOG_CLIENT)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _patch_xray(server_host, mismatch)
        _patch_xray(client_host, mismatch)

        with linux_env.xp2p_run_session_with_env(
            server_host,
            "server",
            helpers.INSTALL_ROOT,
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
            allow_mismatch=True,
            auto_install=False,
        ) as server_session:
            assert server_session["pid"] > 0
            with linux_env.xp2p_run_session_with_env(
                client_host,
                "client",
                helpers.INSTALL_ROOT,
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
                allow_mismatch=True,
                auto_install=False,
            ) as client_session:
                assert client_session["pid"] > 0
            time.sleep(1)
            _assert_mismatch_warned(client_host, "client")
        time.sleep(1)
        _assert_mismatch_warned(server_host, "server")
    finally:
        _restore_xray(server_host)
        _restore_xray(client_host)
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
