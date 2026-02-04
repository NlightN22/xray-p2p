from __future__ import annotations

import json
from pathlib import Path

import pytest

from tests.host.win import _client_runtime, _server_runtime, env as win_env

PINNED_JSON = Path("go/internal/xray/pinned.json")
INSTALL_DIR = win_env.PROGRAM_FILES_INSTALL_DIR
SERVER_CONFIG_NAME = "config-server"
CLIENT_CONFIG_NAME = "config-client"
SERVER_LOG_RELATIVE = r"logs\server.err"
CLIENT_LOG_RELATIVE = r"logs\client.err"
XRAY_PATH = INSTALL_DIR / "bin" / "xray.exe"
XRAY_BACKUP = INSTALL_DIR / "bin" / "xray.pinned.bak"
RUN_OUTPUT_DIR = Path(r"C:\xp2p\build\artifacts\pinned-version")
SERVER_RUN_OUTPUT = RUN_OUTPUT_DIR / "server-run.log"
CLIENT_RUN_OUTPUT = RUN_OUTPUT_DIR / "client-run.log"


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


def _state_files_for(install_dir: Path) -> list[Path]:
    return [
        install_dir / "install-state-client.json",
        install_dir / "install-state-server.json",
        install_dir / "install-state.json",
    ]


def _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner) -> None:
    xp2p_server_runner("server", "remove", "--ignore-missing", "--quiet")
    xp2p_client_runner("client", "remove", "--all", "--ignore-missing", "--quiet")
    win_env.cleanup_xp2p_install(
        server_host,
        config_dirs=[INSTALL_DIR / SERVER_CONFIG_NAME],
        state_files=_state_files_for(INSTALL_DIR),
    )
    win_env.cleanup_xp2p_install(
        client_host,
        config_dirs=[INSTALL_DIR / CLIENT_CONFIG_NAME],
        state_files=_state_files_for(INSTALL_DIR),
    )


def _extract_generated_credential(stdout: str) -> dict[str, str | None]:
    user = password = link = None
    for raw_line in (stdout or "").splitlines():
        line = raw_line.strip()
        lowered = line.lower()
        if lowered.startswith("user:"):
            user = line.split(":", 1)[1].strip()
        elif lowered.startswith("password:"):
            password = line.split(":", 1)[1].strip()
        elif lowered.startswith("link:"):
            link = line.split(":", 1)[1].strip()
    if user is None or password is None:
        pytest.fail(
            "xp2p server install did not emit trojan credential (missing user/password lines).\n"
            f"STDOUT:\n{stdout}"
        )
    if not link:
        pytest.fail("xp2p server install did not emit trojan link.")
    return {"user": user, "password": password, "link": link}


def _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner) -> dict[str, str | None]:
    server_install = xp2p_server_runner(
        "--server-host",
        win_env.DEFAULT_TARGET,
        "server",
        "install",
        "--force",
        check=True,
    )
    credential = _extract_generated_credential(server_install.stdout or "")
    xp2p_client_runner(
        "client",
        "install",
        "--link",
        credential["link"],
        "--force",
        check=True,
    )
    return credential


def _patch_xray(server_host, client_host, expected: str, replacement: str) -> None:
    for host in (server_host, client_host):
        result = win_env.run_guest_script(
            host,
            "scripts/patch_xray_version.ps1",
            XrayPath=str(XRAY_PATH),
            BackupPath=str(XRAY_BACKUP),
            ExpectedVersion=expected,
            ReplacementVersion=replacement,
        )
        if result.rc != 0:
            pytest.fail(
                "Failed to patch xray version.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )


def _restore_xray(server_host, client_host) -> None:
    for host in (server_host, client_host):
        if win_env.path_exists(host, XRAY_BACKUP):
            win_env.run_guest_script(
                host,
                "scripts/copy_file.ps1",
                Source=str(XRAY_BACKUP),
                Destination=str(XRAY_PATH),
            )
            win_env.remove_path(host, XRAY_BACKUP)


def _assert_mismatch_logged(host, path: Path, role: str) -> None:
    content = win_env.read_text(host, path).lower()
    assert "xray version mismatch" in content, f"Expected version mismatch in {role} run log:\n{content}"


def _assert_mismatch_warned(host, path: Path, role: str) -> None:
    content = win_env.read_text(host, path).lower()
    assert "xray version mismatch allowed" in content, (
        f"Expected mismatch warning in {role} run log:\n{content}"
    )


@pytest.mark.host
@pytest.mark.win
def test_xray_pinned_version_allows_matching(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
):
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        with xp2p_server_run_factory(str(INSTALL_DIR), SERVER_CONFIG_NAME, SERVER_LOG_RELATIVE) as server_session:
            assert server_session["pid"] > 0
            with xp2p_client_run_factory(str(INSTALL_DIR), CLIENT_CONFIG_NAME, CLIENT_LOG_RELATIVE) as client_session:
                assert client_session["pid"] > 0
    finally:
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.win
def test_xray_pinned_version_rejects_mismatch(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
):
    pinned = _pinned_version()
    mismatch = _mismatch_version(pinned)
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    win_env.remove_paths(server_host, [SERVER_RUN_OUTPUT])
    win_env.remove_paths(client_host, [CLIENT_RUN_OUTPUT])
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _patch_xray(server_host, client_host, pinned, mismatch)

        server_result = win_env.run_guest_script(
            server_host,
            "scripts/start_xp2p_server_run.ps1",
            Xp2pPath=str(win_env.XP2P_EXE),
            InstallDir=str(INSTALL_DIR),
            ConfigDir=SERVER_CONFIG_NAME,
            LogRelative=SERVER_LOG_RELATIVE,
            LogPath=str(INSTALL_DIR / SERVER_LOG_RELATIVE),
            StabilizeSeconds="6",
            OutputLogPath=str(SERVER_RUN_OUTPUT),
        )
        assert server_result.rc != 0, "Expected server run to fail with mismatched xray"
        _assert_mismatch_logged(server_host, SERVER_RUN_OUTPUT, "server")

        client_result = win_env.run_guest_script(
            client_host,
            "scripts/start_xp2p_client_run.ps1",
            Xp2pPath=str(win_env.XP2P_EXE),
            InstallDir=str(INSTALL_DIR),
            ConfigDir=CLIENT_CONFIG_NAME,
            LogRelative=CLIENT_LOG_RELATIVE,
            LogPath=str(INSTALL_DIR / CLIENT_LOG_RELATIVE),
            StabilizeSeconds="6",
            OutputLogPath=str(CLIENT_RUN_OUTPUT),
        )
        assert client_result.rc != 0, "Expected client run to fail with mismatched xray"
        _assert_mismatch_logged(client_host, CLIENT_RUN_OUTPUT, "client")
    finally:
        _restore_xray(server_host, client_host)
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.win
def test_xray_pinned_version_allows_override(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
):
    pinned = _pinned_version()
    mismatch = _mismatch_version(pinned)
    _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
    win_env.remove_paths(server_host, [SERVER_RUN_OUTPUT])
    win_env.remove_paths(client_host, [CLIENT_RUN_OUTPUT])
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _patch_xray(server_host, client_host, pinned, mismatch)

        with _server_runtime.xp2p_server_run_session_with_env(
            server_host,
            str(INSTALL_DIR),
            SERVER_CONFIG_NAME,
            SERVER_LOG_RELATIVE,
            allow_mismatch=True,
            output_log_path=str(SERVER_RUN_OUTPUT),
        ) as server_session:
            assert server_session["pid"] > 0
            with _client_runtime.xp2p_client_run_session_with_env(
                client_host,
                str(INSTALL_DIR),
                CLIENT_CONFIG_NAME,
                CLIENT_LOG_RELATIVE,
                allow_mismatch=True,
                output_log_path=str(CLIENT_RUN_OUTPUT),
            ) as client_session:
                assert client_session["pid"] > 0

        _assert_mismatch_warned(client_host, CLIENT_RUN_OUTPUT, "client")
        _assert_mismatch_warned(server_host, SERVER_RUN_OUTPUT, "server")
    finally:
        _restore_xray(server_host, client_host)
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
