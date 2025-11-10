from __future__ import annotations

from pathlib import Path

import pytest

from tests.host import _env

MSI_CACHE_DIR = Path(r"C:\xp2p\build\msi-cache")
MSI_MIN_SIZE_BYTES = 1_000_000


@pytest.mark.host
def test_windows_installer_builds_msi(server_host):
    msi_path = _env.ensure_msi_package(server_host)
    assert msi_path.startswith(str(MSI_CACHE_DIR)), (
        f"Expected MSI to be placed under {MSI_CACHE_DIR}, got {msi_path}"
    )

    size_value = _env.get_remote_file_size(server_host, msi_path)
    assert size_value >= MSI_MIN_SIZE_BYTES, (
        f"Expected MSI to be at least {MSI_MIN_SIZE_BYTES} bytes, got {size_value}"
    )
