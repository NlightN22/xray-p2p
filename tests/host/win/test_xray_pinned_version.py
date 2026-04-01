from __future__ import annotations

import json
import time
from pathlib import Path

import pytest

from tests.host.win import _client_runtime, _server_runtime, env as win_env

PINNED_JSON = Path("go/internal/xray/pinned.json")
SERVER_CONFIG_NAME = "config-server"
CLIENT_CONFIG_NAME = "config-client"
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
        win_env.CONFIG_ROOT / "xp2p-client.toml",
        win_env.CONFIG_ROOT / "xp2p-server.toml",
        win_env.CONFIG_ROOT / "xp2p-client.state.json",
        win_env.CONFIG_ROOT / "xp2p-server.state.json",
    ]


def _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner) -> None:
    _stop_xray_processes(server_host)
    _stop_xray_processes(client_host)
    xp2p_server_runner("server", "remove", "--ignore-missing", "--quiet")
    xp2p_client_runner("client", "remove", "--all", "--ignore-missing", "--quiet")
    server_install_dir = _install_dir(server_host)
    client_install_dir = _install_dir(client_host)
    _remove_xray_backup(server_host)
    _remove_xray_backup(client_host)
    win_env.cleanup_xp2p_install(
        server_host,
        config_dirs=[win_env.CONFIG_ROOT / SERVER_CONFIG_NAME],
        state_files=_state_files_for(server_install_dir),
    )
    win_env.cleanup_xp2p_install(
        client_host,
        config_dirs=[win_env.CONFIG_ROOT / CLIENT_CONFIG_NAME],
        state_files=_state_files_for(client_install_dir),
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
        "server",
        "install",
        "--host",
        win_env.DEFAULT_TARGET,
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


def _stop_xray_processes(host) -> None:
    script = """
$ErrorActionPreference = 'SilentlyContinue'
Get-Process -Name xray,'xray.pinned.bak' -ErrorAction SilentlyContinue | ForEach-Object {
    try { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue } catch { }
}
"""
    win_env.run_powershell(host, script)


def _wrap_xray(server_host, client_host, fake_version: str) -> None:
    for host in (server_host, client_host):
        _stop_xray_processes(host)
        result = win_env.run_guest_script(
            host,
            "scripts/wrap_xray_version.ps1",
            XrayPath=str(_xray_path(host)),
            BackupPath=str(_xray_backup(host)),
            FakeVersion=fake_version,
            )
        if result.rc != 0:
            pytest.fail(
                "Failed to wrap xray version.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _run_guest_script_with_retry(host, script: str, *, retries: int = 3, delay: float = 5.0, **params):
    last_exc: Exception | None = None
    for attempt in range(1, retries + 1):
        try:
            return win_env.run_guest_script(host, script, **params)
        except Exception as exc:
            last_exc = exc
            backend = getattr(host, "backend", None)
            if backend is not None and hasattr(backend, "_reset_client"):
                backend._reset_client()
            if attempt < retries:
                time.sleep(delay)
                continue
            raise
    if last_exc is not None:
        raise last_exc


def _restore_xray(server_host, client_host) -> None:
    for host in (server_host, client_host):
        _stop_xray_processes(host)
        backup = _xray_backup(host)
        if win_env.path_exists(host, backup):
            win_env.run_guest_script(
                host,
                "scripts/copy_file.ps1",
                Source=str(backup),
                Destination=str(_xray_path(host)),
                )
            win_env.remove_path(host, backup)


def _remove_xray_backup(host) -> None:
    backup = _xray_backup(host)
    if win_env.path_exists(host, backup):
        win_env.remove_path(host, backup)


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
    server_install_dir = _install_dir(server_host)
    client_install_dir = _install_dir(client_host)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        with xp2p_server_run_factory(
            str(server_install_dir),
            SERVER_CONFIG_NAME,
        ) as server_session:
            assert server_session["pid"] > 0
            with xp2p_client_run_factory(
                str(client_install_dir),
                CLIENT_CONFIG_NAME,
            ) as client_session:
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
    server_install_dir = _install_dir(server_host)
    client_install_dir = _install_dir(client_host)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _wrap_xray(server_host, client_host, mismatch)

        server_result = _run_guest_script_with_retry(
            server_host,
            "scripts/start_xp2p_server_run.ps1",
            Xp2pPath=str(win_env.XP2P_EXE),
            InstallDir=str(server_install_dir),
            ConfigDir=SERVER_CONFIG_NAME,
            StabilizeSeconds="6",
            OutputLogPath=str(SERVER_RUN_OUTPUT),
            )
        _assert_mismatch_logged(server_host, SERVER_RUN_OUTPUT, "server")

        client_result = _run_guest_script_with_retry(
            client_host,
            "scripts/start_xp2p_client_run.ps1",
            Xp2pPath=str(win_env.XP2P_EXE),
            InstallDir=str(client_install_dir),
            ConfigDir=CLIENT_CONFIG_NAME,
            StabilizeSeconds="6",
            OutputLogPath=str(CLIENT_RUN_OUTPUT),
            )
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
    server_install_dir = _install_dir(server_host)
    client_install_dir = _install_dir(client_host)
    try:
        _install_server_client(server_host, client_host, xp2p_server_runner, xp2p_client_runner)
        _wrap_xray(server_host, client_host, mismatch)

        with _server_runtime.xp2p_server_run_session_with_env(
            server_host,
            str(server_install_dir),
            SERVER_CONFIG_NAME,
            allow_mismatch=True,
            output_log_path=str(SERVER_RUN_OUTPUT),
        ) as server_session:
            assert server_session["pid"] > 0
            with _client_runtime.xp2p_client_run_session_with_env(
                client_host,
                str(client_install_dir),
                CLIENT_CONFIG_NAME,
                allow_mismatch=True,
                output_log_path=str(CLIENT_RUN_OUTPUT),
            ) as client_session:
                assert client_session["pid"] > 0

        _assert_mismatch_warned(client_host, CLIENT_RUN_OUTPUT, "client")
        _assert_mismatch_warned(server_host, SERVER_RUN_OUTPUT, "server")
    finally:
        _restore_xray(server_host, client_host)
        _cleanup_install(server_host, client_host, xp2p_server_runner, xp2p_client_runner)


def _install_dir(host) -> Path:
    return win_env.get_program_files_install_dir(host)


def _xray_path(host) -> Path:
    return _install_dir(host) / "bin" / "xray.exe"


def _xray_backup(host) -> Path:
    return _install_dir(host) / "bin" / "xray.pinned.bak"

