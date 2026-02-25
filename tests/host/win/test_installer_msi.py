from __future__ import annotations

from pathlib import Path

import pytest

from tests.host import config_files
from tests.host.win import env as _env

MSI_CACHE_DIR_X64 = _env.MSI_CACHE_DIR_X64
MSI_CACHE_DIR_X86 = _env.MSI_CACHE_DIR_X86
MSI_MIN_SIZE_BYTES = 1_000_000


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_builds_msi(server_host, pytestconfig: pytest.Config):
    if not pytestconfig.getoption("run_msi_build_tests"):
        pytest.skip("MSI build tests are skipped by default.")
    msi_path = _env.ensure_msi_package(server_host)
    assert msi_path.startswith(str(MSI_CACHE_DIR_X64)), (
        f"Expected MSI to be placed under {MSI_CACHE_DIR_X64}, got {msi_path}"
    )

    size_value = _env.get_remote_file_size(server_host, msi_path)
    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_builds_msi_x86(server_host, pytestconfig: pytest.Config):
    if not pytestconfig.getoption("run_msi_build_tests"):
        pytest.skip("MSI build tests are skipped by default.")
    msi_path = _env.ensure_msi_package_x86(server_host)
    assert msi_path.startswith(str(MSI_CACHE_DIR_X86)), (
        f"Expected x86 MSI to be placed under {MSI_CACHE_DIR_X86}, got {msi_path}"
    )

    size_value = _env.get_remote_file_size(server_host, msi_path)
    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected x86 MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_places_xray_binary(server_host, xp2p_msi_path):
    install_root = _install_root(server_host)
    xray_path = install_root / "bin" / "xray.exe"
    assert _remote_path_exists(server_host, xray_path), (
        f"Expected xray binary at {xray_path}"
    )


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_preserves_config_files(server_host, xp2p_msi_path, xp2p_server_runner):
    install_dir = _install_root(server_host)
    client_dir = _env.CONFIG_ROOT / "config-client"
    server_dir = _env.CONFIG_ROOT / "config-server"
    state_files = [
        _env.CONFIG_ROOT / "xp2p-client.toml",
        _env.CONFIG_ROOT / "xp2p-server.toml",
        _env.CONFIG_ROOT / "xp2p-client.state.json",
        _env.CONFIG_ROOT / "xp2p-server.state.json",
    ]
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[client_dir, server_dir],
        state_files=state_files,
    )

    xp2p_server_runner(
        "client",
        "install",
        "--path",
        str(install_dir),
        "--config-dir",
        "config-client",
        "--host",
        "10.55.10.10",
        "--user",
        "cfg-client@example.com",
        "--password",
        "cfg-client-secret",
        "--force",
        check=True,
    )
    xp2p_server_runner(
        "server",
        "install",
        "--path",
        str(install_dir),
        "--config-dir",
        "config-server",
        "--port",
        "62122",
        "--host",
        "cfg-server.example.com",
        "--force",
        check=True,
    )

    client_files = config_files.config_paths(client_dir, config_files.CLIENT_CONFIG_FILES)
    server_files = config_files.config_paths(server_dir, config_files.SERVER_CONFIG_FILES)
    _assert_paths_exist(server_host, client_files + server_files)

    try:
        _env.uninstall_xp2p_from_msi(server_host, xp2p_msi_path, purge_files=False)
        _assert_paths_exist(server_host, client_files + server_files)
    finally:
        _env.install_xp2p_from_msi(server_host, xp2p_msi_path)
        _env.cleanup_xp2p_install(
            server_host,
            config_dirs=[client_dir, server_dir],
            state_files=state_files,
        )


def _remote_path_exists(host, path: Path) -> bool:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (Test-Path {quoted}) {{
    exit 0
}}
exit 3
"""
    result = _env.run_powershell(host, script)
    return result.rc == 0


def _install_root(host) -> Path:
    return _env.get_program_files_install_dir(host)


def _assert_paths_exist(host, paths: list[Path]) -> None:
    expected = {str(path) for path in paths}
    existing = _env.paths_exist(host, paths)
    missing = sorted(expected - existing)
    if missing:
        rendered = "\n".join(missing)
        pytest.fail(f"Expected config files to exist:\n{rendered}")
