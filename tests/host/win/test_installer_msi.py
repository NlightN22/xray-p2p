from __future__ import annotations

import pytest

from tests.host.win import env as _env

MSI_ARTIFACTS_DIR_X64 = _env.MSI_ARTIFACTS_DIR_X64
MSI_ARTIFACTS_DIR_X86 = _env.MSI_ARTIFACTS_DIR_X86
INSTALL_ROOT = _env.PROGRAM_FILES_INSTALL_DIR
XRAY_PATH = INSTALL_ROOT / "bin" / "xray.exe"
MSI_MIN_SIZE_BYTES = 1_000_000


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_builds_msi(builder_host):
    latest_path = _env.ensure_msi_package(builder_host)
    msi_path = _env.get_msi_path_from_latest(builder_host, latest_path)
    assert msi_path.startswith(str(MSI_ARTIFACTS_DIR_X64)), (
        f"Expected MSI to be placed under {MSI_ARTIFACTS_DIR_X64}, got {msi_path}"
    )

    size_value = _env.get_remote_file_size(builder_host, msi_path)
    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_builds_msi_x86(builder_host):
    latest_path = _env.ensure_msi_package_x86(builder_host)
    msi_path = _env.get_msi_path_from_latest(builder_host, latest_path)
    assert msi_path.startswith(str(MSI_ARTIFACTS_DIR_X86)), (
        f"Expected x86 MSI to be placed under {MSI_ARTIFACTS_DIR_X86}, got {msi_path}"
    )

    size_value = _env.get_remote_file_size(builder_host, msi_path)
    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected x86 MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )


@pytest.mark.host
@pytest.mark.win
def test_windows_installer_places_xray_binary(server_host, xp2p_msi_path):
    assert _env.path_exists(server_host, XRAY_PATH), (
        f"Expected xray binary at {XRAY_PATH}"
    )
