from __future__ import annotations

from pathlib import Path
import time
from contextlib import contextmanager

import pytest

from tests.host.win import env as _env
DEFAULT_SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
DEFAULT_SERVER_CONFIG_NAME = "config-server"
DEFAULT_CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
DEFAULT_CLIENT_CONFIG_NAME = "config-client"
SERVER_LOG_RELATIVE = r"logs\server.err"
CLIENT_LOG_RELATIVE = r"logs\client.err"
DEFAULT_DIAGNOSTICS_PORT = 62022

CUSTOM_SERVER_INSTALL_DIR = Path(r"C:\ProgramData\xp2p-it\server")
CUSTOM_SERVER_CONFIG_NAME = "it-server-config"
CUSTOM_CLIENT_INSTALL_DIR = Path(r"C:\ProgramData\xp2p-it\client")
CUSTOM_CLIENT_CONFIG_NAME = "it-client-config"
CUSTOM_SERVER_PORT = 62055
CUSTOM_SERVER_HOST = "xp2p-integration.local"
CUSTOM_CERT_PATH = Path(r"C:\xp2p\tests\fixtures\tls\integration-cert.pem")
CUSTOM_KEY_PATH = Path(r"C:\xp2p\tests\fixtures\tls\integration-key.pem")
XRAY_SOURCE_X64 = Path(r"C:\xp2p\distro\windows\bundle\x86_64\xray.exe")
WINTUN_SOURCE_X64 = Path(r"C:\xp2p\distro\windows\bundle\x86_64\wintun.dll")
DEFAULT_SERVER_TUN = "xp2ps"
DEFAULT_CLIENT_TUN = "xp2pc"


@contextmanager
def _timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def _server_public_host() -> str:
    return _env.DEFAULT_TARGET


def _remove_remote_path(host, path: Path) -> None:
    _env.remove_paths(host, [path])


def _resolve_server_config_dir(install_dir: Path | None) -> Path:
    if install_dir == CUSTOM_SERVER_INSTALL_DIR:
        return _env.CONFIG_ROOT / CUSTOM_SERVER_CONFIG_NAME
    return _env.CONFIG_ROOT / DEFAULT_SERVER_CONFIG_NAME


def _resolve_client_config_dir(install_dir: Path | None) -> Path:
    if install_dir == CUSTOM_CLIENT_INSTALL_DIR:
        return _env.CONFIG_ROOT / CUSTOM_CLIENT_CONFIG_NAME
    return _env.CONFIG_ROOT / DEFAULT_CLIENT_CONFIG_NAME


def _state_files_for(install_dir: Path) -> list[Path]:
    return [
        _env.CONFIG_ROOT / "xp2p-client.toml",
        _env.CONFIG_ROOT / "xp2p-server.toml",
        _env.CONFIG_ROOT / "xp2p-client.state.json",
        _env.CONFIG_ROOT / "xp2p-server.state.json",
    ]


def _cleanup_server_install(
    server_host, runner, msi_path: str, install_dir: Path | None = None, purge: bool = False
) -> None:
    with _timed("cleanup server install"):
        args = ["server", "remove", "--ignore-missing"]
        if install_dir is not None:
            args.extend(["--path", str(install_dir)])
        runner(*args)
        target_dir = install_dir or DEFAULT_SERVER_INSTALL_DIR
        _env.cleanup_xp2p_install(
            server_host,
            config_dirs=[_resolve_server_config_dir(target_dir)],
            state_files=_state_files_for(target_dir),
        )
        if purge and install_dir is not None:
            _remove_remote_path(server_host, install_dir)


def _cleanup_client_install(
    client_host, runner, msi_path: str, install_dir: Path | None = None, purge: bool = False
) -> None:
    with _timed("cleanup client install"):
        args = ["client", "remove", "--all", "--ignore-missing"]
        if install_dir is not None:
            args.extend(["--path", str(install_dir)])
        runner(*args)
        target_dir = install_dir or DEFAULT_CLIENT_INSTALL_DIR
        _env.cleanup_xp2p_install(
            client_host,
            config_dirs=[_resolve_client_config_dir(target_dir)],
            state_files=_state_files_for(target_dir),
        )
        if purge and install_dir is not None:
            _remove_remote_path(client_host, install_dir)


def _stage_xray_binary(host, install_dir: Path) -> None:
    target_dir = install_dir / "bin"
    script = f"""
$ErrorActionPreference = 'Stop'
$xraySource = {_env.ps_quote(str(XRAY_SOURCE_X64))}
$wintunSource = {_env.ps_quote(str(WINTUN_SOURCE_X64))}
if (-not (Test-Path $xraySource)) {{
    throw "xray.exe not found at $xraySource"
}}
if (-not (Test-Path $wintunSource)) {{
    throw "wintun.dll not found at $wintunSource"
}}
$destDir = {_env.ps_quote(str(target_dir))}
$xrayDest = Join-Path $destDir 'xray.exe'
$wintunDest = Join-Path $destDir 'wintun.dll'
New-Item -ItemType Directory -Path $destDir -Force | Out-Null
Copy-Item -Path $xraySource -Destination $xrayDest -Force
Copy-Item -Path $wintunSource -Destination $wintunDest -Force
"""
    with _timed("stage xray binary"):
        result = _env.run_powershell(host, script)
        if result.rc != 0:
            pytest.fail(
                "Failed to stage xray.exe prior to custom install.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
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
    return {"user": user, "password": password, "link": link}


def _assert_ping_success(result) -> None:
    assert result.rc == 0, (
        "xp2p ping failed:\n"
        f"STDOUT:\n{result.stdout}\n"
        f"STDERR:\n{result.stderr}"
    )
    output_lower = (result.stdout or "").lower()
    if "100% loss" in output_lower:
        pytest.fail(
            "xp2p ping reported 100% packet loss:\n"
            f"{result.stdout}"
        )
    stderr_text = (result.stderr or "").strip()
    if stderr_text:
        stderr_lower = stderr_text.lower()
        if stderr_lower.startswith("#< clixml"):
            if "level=error" in stderr_lower or "level=warn" in stderr_lower:
                pytest.fail(
                    "xp2p ping reported warnings/errors in STDERR:\n"
                    f"{result.stderr}"
                )
        else:
            pytest.fail(
                "xp2p ping wrote unexpected output to STDERR:\n"
                f"{result.stderr}"
            )


def _assert_ipv6_binding_disabled(host, interface_name: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/assert_ipv6_binding_disabled.ps1",
        InterfaceName=interface_name,
    )
    if result.rc != 0:
        pytest.fail(
            "IPv6 binding check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _run_ping_via_socks(xp2p_client_runner, host: str, port: int | None = None, attempts: int = 3):
    with _timed("ping via socks"):
        args = [
            "ping",
            host,
            "--count",
            str(attempts),
            "--tunnel",
        ]
        if port is not None:
            args[2:2] = ["--port", str(port)]
        return xp2p_client_runner(*args, check=True)


@pytest.mark.host
@pytest.mark.win
def test_install_server_and_client_default(
    server_host,
    client_host,
    xp2p_client_runner,
    xp2p_server_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
    xp2p_msi_path,
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path, DEFAULT_SERVER_INSTALL_DIR)
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path, DEFAULT_CLIENT_INSTALL_DIR)
    try:
        server_public_host = _server_public_host()
        server_install = xp2p_server_runner(
            "--server-host",
            server_public_host,
            "server",
            "install",
            "--force",
            check=True,
        )
        credential = _extract_generated_credential(server_install.stdout or "")
        assert credential["link"], "Trojan link was not emitted for default install"
        assert credential["link"].startswith("trojan://")

        server_session_cm = xp2p_server_run_factory(
            str(DEFAULT_SERVER_INSTALL_DIR),
            DEFAULT_SERVER_CONFIG_NAME,
            SERVER_LOG_RELATIVE,
        )
        with server_session_cm as server_session:
            assert server_session["pid"] > 0

            xp2p_client_runner(
                "client",
                "install",
                "--link",
                credential["link"],
                "--force",
                check=True,
            )

            client_session_cm = xp2p_client_run_factory(
                str(DEFAULT_CLIENT_INSTALL_DIR),
                DEFAULT_CLIENT_CONFIG_NAME,
                CLIENT_LOG_RELATIVE,
            )
            with client_session_cm as client_session:
                assert client_session["pid"] > 0
                _assert_ipv6_binding_disabled(server_host, DEFAULT_SERVER_TUN)
                _assert_ipv6_binding_disabled(client_host, DEFAULT_CLIENT_TUN)
                ping_result = _run_ping_via_socks(xp2p_client_runner, server_public_host)
                _assert_ping_success(ping_result)
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path, DEFAULT_CLIENT_INSTALL_DIR)
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path, DEFAULT_SERVER_INSTALL_DIR)


@pytest.mark.host
@pytest.mark.win
def test_install_server_and_client_nodefault(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
    xp2p_msi_path,
):
    _cleanup_server_install(
        server_host, xp2p_server_runner, xp2p_msi_path, CUSTOM_SERVER_INSTALL_DIR, purge=True
    )
    _cleanup_client_install(
        client_host, xp2p_client_runner, xp2p_msi_path, CUSTOM_CLIENT_INSTALL_DIR, purge=True
    )
    try:
        _stage_xray_binary(server_host, CUSTOM_SERVER_INSTALL_DIR)
        server_public_host = _server_public_host()
        server_install = xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(CUSTOM_SERVER_INSTALL_DIR),
            "--config-dir",
            CUSTOM_SERVER_CONFIG_NAME,
            "--port",
            str(CUSTOM_SERVER_PORT),
            "--host",
            CUSTOM_SERVER_HOST,
            "--cert",
            str(CUSTOM_CERT_PATH),
            "--key",
            str(CUSTOM_KEY_PATH),
            "--force",
            check=True,
        )
        credential = _extract_generated_credential(server_install.stdout or "")

        server_session_cm = xp2p_server_run_factory(
            str(CUSTOM_SERVER_INSTALL_DIR),
            CUSTOM_SERVER_CONFIG_NAME,
            SERVER_LOG_RELATIVE,
        )
        with server_session_cm as server_session:
            assert server_session["pid"] > 0

            _stage_xray_binary(client_host, CUSTOM_CLIENT_INSTALL_DIR)
            xp2p_client_runner(
                "client",
                "install",
                "--path",
                str(CUSTOM_CLIENT_INSTALL_DIR),
                "--config-dir",
                CUSTOM_CLIENT_CONFIG_NAME,
                "--host",
                server_public_host,
                "--port",
                str(CUSTOM_SERVER_PORT),
                "--user",
                credential["user"],
                "--password",
                credential["password"],
                "--sni",
                CUSTOM_SERVER_HOST,
                "--allow-insecure",
                "--force",
                check=True,
            )

            client_session_cm = xp2p_client_run_factory(
                str(CUSTOM_CLIENT_INSTALL_DIR),
                CUSTOM_CLIENT_CONFIG_NAME,
                CLIENT_LOG_RELATIVE,
            )
            with client_session_cm as client_session:
                assert client_session["pid"] > 0
                _assert_ipv6_binding_disabled(server_host, DEFAULT_SERVER_TUN)
                _assert_ipv6_binding_disabled(client_host, DEFAULT_CLIENT_TUN)
                ping_result = _run_ping_via_socks(xp2p_client_runner, server_public_host)
                _assert_ping_success(ping_result)
    finally:
        _cleanup_client_install(
            client_host, xp2p_client_runner, xp2p_msi_path, CUSTOM_CLIENT_INSTALL_DIR, purge=True
        )
        _cleanup_server_install(
            server_host, xp2p_server_runner, xp2p_msi_path, CUSTOM_SERVER_INSTALL_DIR, purge=True
        )
